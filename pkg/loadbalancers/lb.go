package loadbalancers

import (
	"net/http"
	"time"

	"github.com/crossedbot/common/golang/logger"

	"github.com/crossedbot/simpleloadbalancer/pkg/networks"
	"github.com/crossedbot/simpleloadbalancer/pkg/services"
	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
)

type StopFn func()

type LoadBalancer interface {
	AddTarget(target targets.Target, to time.Duration) error

	HealthCheck(interval time.Duration) StopFn

	GC() StopFn

	Start(laddr, protocol string) (StopFn, error)

	Type() string
}

type appLoadBalancer struct {
	Pool services.ServicePool
}

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

type netLoadBalancer struct {
	Pool networks.NetworkPool
}

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
