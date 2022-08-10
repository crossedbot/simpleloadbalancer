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

// networkTargetr represents a network level target; tracking its own reverse
// proxy.
type networkTarget struct {
	Target       targets.Target
	NetworkProxy ReverseNetworkProxy
}

// NetworkPool represents an interface to a Network level service pool for TCP,
// UDP, etc.
type NetworkPool interface {
	// AddTarget adds a given target to the pool and sets the connection
	// timeout.
	AddTarget(target targets.Target, to time.Duration) error

	// HealthCheck starts a service health check routine and returns a stop
	// function that can be called to exit this routine.
	HealthCheck(interval time.Duration) StopFn

	// LoadBalancer starts a listener on the given local address and network
	// protocol and forwards any connections to the backend targets. It uses
	// a Round Robin routing strategy and returns a stop function to stop
	// the listener routine.
	LoadBalancer(laddr, network string) (StopFn, error)
}

// networkPool implements the NetworkPool service and tracks the backend targets
// and the index of the current targeted service.
type networkPool struct {
	Index   uint64
	Targets []*networkTarget
}

// New returns a new NetworkPool.
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
	pool.Targets = append(pool.Targets, &networkTarget{
		Target:       target,
		NetworkProxy: rproxy,
	})
	return nil
}

// AttemptNextTarget attempts the next target to fullfil the given connection
// and returns true if an attempt was made. Otherwise false is returned and we
// reached the maximum attempts or the next target isn't set.
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

// CurrentTarget returns the target at the pool's current index.
func (pool *networkPool) CurrentTarget() *networkTarget {
	idx := int(pool.Index) % len(pool.Targets)
	return pool.Targets[idx]
}

// HandleConnection acts like http.ServeHTTP and handles new connections
// accepted by a listener.
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

// NextIndex returns the next index for the pool; setting what is returned as
// the current index in the process.
func (pool *networkPool) NextIndex() int {
	return int(atomic.AddUint64(&pool.Index, uint64(1)) %
		uint64(len(pool.Targets)))
}

// NextTarget returns the next network target and sets it as the current target.
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

// RetryTarget retries the current network target TargetMaxRetries number of
// times. If the target was retried, true is returned. Otherwise, false is
// returned indicating that the max retries has been reached or the current
// target is not set.
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

// getAttemptsFromContext returns the number of attempts set for a given
// connection context.
func getAttemptsFromContext(ctx context.Context) int {
	attempts, ok := ctx.Value(TargetContextAttemptKey).(int)
	if ok {
		return attempts
	}
	return 0
}

// getRetriesFromContext returns the number of retries set for a given
// connection context.
func getRetriesFromContext(ctx context.Context) int {
	retries, ok := ctx.Value(TargetContextRetryKey).(int)
	if ok {
		return retries
	}
	return 0
}

// getTargetProtocol returns the given target's network protocol. If the
// protocol can not be matched, an empty string is returned instead.
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
