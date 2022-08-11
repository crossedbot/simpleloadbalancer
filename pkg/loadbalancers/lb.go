package loadbalancers

import (
	"net/http"
	"time"

	"github.com/crossedbot/common/golang/logger"

	"github.com/crossedbot/simpleloadbalancer/pkg/networks"
	"github.com/crossedbot/simpleloadbalancer/pkg/services"
	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
)

// StopFn is a prototype for a stop routine function.
type StopFn func()

// LoadBalancer represents a common interface for all load balancer types.
type LoadBalancer interface {
	// AddTarget adds the given target to the load balancer with the given
	// connection timeout.
	AddTarget(target targets.Target, to time.Duration) error

	// HealthCheck starts a routine to passively track the health of the the
	// LB's targets. It returns stop function to stop the health check
	// routine.
	HealthCheck(interval time.Duration) StopFn

	// GC starts the IP registry garbage collector and returns a stop
	// function to stop this routine.
	GC() StopFn

	// Start starts the load balancer on the given listening address and
	// protocol. It returns a stop function to stop listening and exit the
	// routine.
	Start(laddr, protocol string) (StopFn, error)

	// Type returns the string representation of the load balancer's type;
	// this is the long name.
	Type() string
}

// appLoadBalancer implements the LoadBalancer interface as application load
// balancer and manages an internal service pool. Application means HTTP
// services.
type appLoadBalancer struct {
	Pool services.ServicePool
}

// NewApplicationLoadBalancer returns a new Load Balancer for targeted HTTP
// services.
func NewApplicationLoadBalancer(reqRate time.Duration, reqCap int64) LoadBalancer {
	return &appLoadBalancer{Pool: services.New(int64(reqRate), reqCap)}
}

func (alb *appLoadBalancer) AddTarget(target targets.Target, to time.Duration) error {
	return alb.Pool.AddService(target)
}

func (alb *appLoadBalancer) HealthCheck(interval time.Duration) StopFn {
	return StopFn(alb.Pool.HealthCheck(interval))
}

func (alb *appLoadBalancer) GC() StopFn {
	return StopFn(alb.Pool.GC())
}

func (alb *appLoadBalancer) Start(laddr, protocol string) (StopFn, error) {
	server := http.Server{
		Addr:    laddr,
		Handler: http.HandlerFunc(alb.Pool.LoadBalancer()),
	}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			logger.Error(err)
		}
	}()
	return func() { server.Close() }, nil
}

func (alb *appLoadBalancer) Type() string {
	return LoadBalancerTypeApp.Long()
}

// netLoadBalancer implements the LoadBalancer interface as a network (E.g. TCP,
// UDP, etc.) load balancer and manages its own network pool.
type netLoadBalancer struct {
	Pool networks.NetworkPool
}

// NewNetworkLoadBalancer returns a LoadBalancer for network-level targets. This
// means services that expect TCP, UDP, whatever connections.
func NewNetworkLoadBalancer() LoadBalancer {
	return &netLoadBalancer{Pool: networks.New()}
}

func (nlb *netLoadBalancer) AddTarget(target targets.Target, to time.Duration) error {
	return nlb.Pool.AddTarget(target, to)
}

func (nlb *netLoadBalancer) HealthCheck(interval time.Duration) StopFn {
	return StopFn(nlb.Pool.HealthCheck(interval))
}

func (nlb *netLoadBalancer) GC() StopFn {
	return StopFn(func() {})
}

func (nlb *netLoadBalancer) Start(laddr, protocol string) (StopFn, error) {
	stopFn, err := nlb.Pool.LoadBalancer(laddr, protocol)
	return StopFn(stopFn), err
}

func (nlb *netLoadBalancer) Type() string {
	return LoadBalancerTypeNet.Long()
}
