package networks

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/crossedbot/common/golang/logger"

	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
)

const (
	// Target constants
	TargetMaxAttempts   = 3
	TargetMaxRetries    = 3
	TargetRetryInterval = 100 * time.Millisecond

	// Context keys
	TargetContextAttemptKey = iota + 1
	TargetContextRetryKey
)

var (
	// Errors
	ErrUnsupportedProtocol = errors.New("Protocol not supported")
	ErrExhaustedTargets    = errors.New("Network targets exhausted")
	ErrTargetMissingHost   = errors.New("Target is missing host value")
	ErrTargetMissingPort   = errors.New("Target is missing port value")
)

// XXX prob put in a common place
type StopFn func()

type networkTarget struct {
	Target       targets.Target
	NetworkProxy ReverseNetworkProxy
}

type NetworkPool interface {
	AddTarget(target targets.Target, to time.Duration) error

	HealthCheck(interval time.Duration) StopFn

	LoadBalancer(laddr, network string) (StopFn, error)
}

type networkPool struct {
	Index   uint64
	Targets []*networkTarget
}

func New() NetworkPool {
	return &networkPool{}
}

func (pool *networkPool) AddTarget(target targets.Target, to time.Duration) error {
	proto := getTargetProtocol(target)
	if proto == "" {
		return ErrUnsupportedProtocol
	}
	host := target.Get("host")
	if host == "" {
		return ErrTargetMissingHost
	}
	port := target.Get("port")
	if port == "" {
		return ErrTargetMissingPort
	}
	hostPort := net.JoinHostPort(host, port)
	rproxy := NewReverseNetworkProxy(proto, hostPort, to)
	rproxy.SetErrorHandler(
		func(ctx context.Context, conn net.Conn, err error) {
			logger.Error(fmt.Sprintf("%s (%s)",
				err, conn.RemoteAddr().String()))
			alive := pool.RetryTarget(ctx, conn)
			target.SetAlive(alive)
			if !alive && !pool.AttemptNextTarget(ctx, conn) {
				logger.Error(fmt.Sprintf("%s (%s)",
					ErrExhaustedTargets.Error(),
					conn.RemoteAddr().String()))
				_, cancelCtx := context.WithCancel(ctx)
				cancelCtx()
				conn.Close()
			}
		},
	)
	// XXX Temp. debugging
	rproxy.SetDebug(true)
	pool.Targets = append(pool.Targets, &networkTarget{
		Target:       target,
		NetworkProxy: rproxy,
	})
	return nil
}

func (pool *networkPool) AttemptNextTarget(ctx context.Context, conn net.Conn) bool {
	attempts := getAttemptsFromContext(ctx)
	if attempts < TargetMaxAttempts {
		target := pool.NextTarget()
		if target == nil {
			return false
		}
		ctx = context.WithValue(ctx, TargetContextAttemptKey,
			attempts+1)
		target.NetworkProxy.Proxy(ctx, conn)
		return true
	}
	return false
}

func (pool *networkPool) CurrentTarget() *networkTarget {
	idx := int(pool.Index) % len(pool.Targets)
	return pool.Targets[idx]
}

func (pool *networkPool) HandleConnection(conn net.Conn) {
	ctx := context.Background()
	pool.AttemptNextTarget(ctx, conn)
}

func (pool *networkPool) HealthCheck(interval time.Duration) StopFn {
	quit := make(chan struct{})
	t := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-quit:
				t.Stop()
				return
			case <-t.C:
				for _, target := range pool.Targets {
					alive := target.Target.IsAvailable(
						3 * time.Second)
					target.Target.SetAlive(alive)
				}
			}
		}
	}()
	return func() { close(quit) }
}

func (pool *networkPool) LoadBalancer(laddr, network string) (StopFn, error) {
	quit := make(chan struct{})
	listener, err := net.Listen("tcp", laddr)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			select {
			case <-quit:
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					logger.Error(err)
					continue
				}
				go pool.HandleConnection(conn)
			}
		}
	}()
	return func() { close(quit) }, nil
}

func (pool *networkPool) NextIndex() int {
	return int(atomic.AddUint64(&pool.Index, uint64(1)) %
		uint64(len(pool.Targets)))
}

func (pool *networkPool) NextTarget() *networkTarget {
	next := pool.NextIndex()
	cycle := len(pool.Targets) + next
	for i := next; i < cycle; i++ {
		idx := i % len(pool.Targets)
		if pool.Targets[idx].Target.IsAlive() {
			if i != next {
				atomic.StoreUint64(&pool.Index, uint64(idx))
			}
			return pool.Targets[idx]
		}
	}
	return nil
}

func (pool *networkPool) RetryTarget(ctx context.Context, conn net.Conn) bool {
	retries := getRetriesFromContext(ctx)
	after := time.After(TargetRetryInterval)
	for retries < TargetMaxRetries {
		select {
		case <-after:
			target := pool.CurrentTarget()
			if target == nil {
				return false
			}
			ctx := context.WithValue(ctx, TargetContextRetryKey,
				retries+1)
			target.NetworkProxy.Proxy(ctx, conn)
			return true
		}
	}
	return false
}

func getAttemptsFromContext(ctx context.Context) int {
	attempts, ok := ctx.Value(TargetContextAttemptKey).(int)
	if ok {
		return attempts
	}
	return 0
}

func getRetriesFromContext(ctx context.Context) int {
	retries, ok := ctx.Value(TargetContextRetryKey).(int)
	if ok {
		return retries
	}
	return 0
}

func getTargetProtocol(target targets.Target) string {
	proto := ""
	targetProto := target.Get("protocol")
	for _, v := range []string{"tcp", "udp"} {
		if strings.EqualFold(targetProto, v) {
			proto = targetProto
			break
		}
	}
	if proto == "" {
		// Try to get the transport from the list if we couldn't match
		// one
		protos := targets.GetTransport(targetProto)
		if protos != nil && len(protos) > 0 {
			return protos[0]
		}
	}
	return proto
}
