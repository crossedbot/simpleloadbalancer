package loadbalancers

import (
	"context"
	"net/http"
	"time"

	"github.com/crossedbot/common/golang/logger"

	"github.com/crossedbot/simpleloadbalancer/pkg/networks"
	"github.com/crossedbot/simpleloadbalancer/pkg/rules"
	"github.com/crossedbot/simpleloadbalancer/pkg/services"
	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
)

// StopFn is a prototype for a stop routine function.
type StopFn func()

// LoadBalancer represents a common interface for all load balancer types.
type LoadBalancer interface {
	// AddTargetGroup adds the given target group to the load balancer. For
	// network load balancers, there is a single target group. Any
	// additional target groups added to a NLB will simply append the
	// targets to the existing group.
	AddTargetGroup(group *targets.TargetGroup) error

	// HealthCheck starts a routine to passively track the health of the
	// each LB target. It returns a stop function to stop the health check
	// each target's health check routine.
	HealthCheck(interval time.Duration) StopFn

	// GC starts the IP registry garbage collector for each LB target and
	// returns a stop function to stop these routines.
	GC() StopFn

	// Start starts the load balancer on the given listening address and
	// protocol. It returns a stop function to stop listening and exit the
	// routine.
	Start(laddr, protocol string) (StopFn, error)

	// Type returns the string representation of the load balancer's type;
	// this is the long name.
	Type() string
}

// appTarget is mapping of an ALB's service pool and other informational fields
// like a name and targeting rules.
type appTarget struct {
	Name string               // Target name
	Rule rules.Rule           // Listener rule
	Pool services.ServicePool // Service pool
}

// appLoadBalancer implements the LoadBalancer interface as application load
// balancer and manages an internal service pool. Application means HTTP
// services.
type appLoadBalancer struct {
	Rate     int64       // Request Rate
	Capacity int64       // Request capacity
	Targets  []appTarget // Service targets
}

// NewApplicationLoadBalancer returns a new Load Balancer for targeted HTTP
// services.
func NewApplicationLoadBalancer(reqRate time.Duration, reqCap int64) LoadBalancer {
	return &appLoadBalancer{
		Rate:     int64(reqRate),
		Capacity: int64(reqCap),
	}
}

func (alb *appLoadBalancer) AddTargetGroup(group *targets.TargetGroup) error {
	pool := services.New(alb.Rate, alb.Capacity)
	for _, t := range group.Targets {
		if err := pool.AddService(t); err != nil {
			return err
		}
	}
	alb.Targets = append(alb.Targets, appTarget{
		Name: group.Name,
		Rule: group.Rule,
		Pool: pool,
	})
	return nil
}

func (alb *appLoadBalancer) HealthCheck(interval time.Duration) StopFn {
	stops := []StopFn{}
	for _, t := range alb.Targets {
		stops = append(stops, StopFn(t.Pool.HealthCheck(interval)))
	}
	return func() {
		for _, fn := range stops {
			fn()
		}
	}
}

func (alb *appLoadBalancer) GC() StopFn {
	stops := []StopFn{}
	for _, t := range alb.Targets {
		stops = append(stops, StopFn(t.Pool.GC()))
	}
	return func() {
		for _, fn := range stops {
			fn()
		}
	}
}

func (alb *appLoadBalancer) Start(laddr, protocol string) (StopFn, error) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		for _, t := range alb.Targets {
			if t.Rule.Matches(r) {
				t.Pool.LoadBalancer()(w, r)
			}
		}
	}
	server := http.Server{
		Addr:    laddr,
		Handler: http.HandlerFunc(handler),
	}
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logger.Error(err)
		}
	}()
	return func() { server.Shutdown(context.Background()) }, nil
}

func (alb *appLoadBalancer) Type() string {
	return LoadBalancerTypeApp.Long()
}

// netLoadBalancer implements the LoadBalancer interface as a network (E.g. TCP,
// UDP, etc.) load balancer and manages its own network pool.
type netLoadBalancer struct {
	Pool    networks.NetworkPool
	Timeout time.Duration
}

// NewNetworkLoadBalancer returns a LoadBalancer for network-level targets. This
// means services that expect TCP, UDP, whatever connections.
func NewNetworkLoadBalancer(to time.Duration) LoadBalancer {
	return &netLoadBalancer{
		Pool:    networks.New(),
		Timeout: to,
	}
}

func (nlb *netLoadBalancer) AddTargetGroup(group *targets.TargetGroup) error {
	for _, t := range group.Targets {
		if err := nlb.Pool.AddTarget(t, nlb.Timeout); err != nil {
			return err
		}
	}
	return nil
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
