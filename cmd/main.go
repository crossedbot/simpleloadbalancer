package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/crossedbot/common/golang/logger"
	"github.com/crossedbot/common/golang/service"

	"github.com/crossedbot/simpleloadbalancer/pkg/loadbalancers"
	"github.com/crossedbot/simpleloadbalancer/pkg/rules"
	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
)

const (
	// Exit codes
	FATAL_EXITCODE = iota + 1
)

// fatal logs the given format string and arguments as an error and exits with
// FATAL_EXITCODE.
func fatal(format string, a ...interface{}) {
	logger.Error(fmt.Errorf(format, a...))
	os.Exit(FATAL_EXITCODE)
}

// newLb returns a new LoadBalancer using the given configuration.
func newLb(c Config) (loadbalancers.LoadBalancer, error) {
	var lb loadbalancers.LoadBalancer
	lbType := loadbalancers.Type(c.Type)
	switch lbType {
	case loadbalancers.LoadBalancerTypeApp:
		rate := time.Duration(c.RequestRate) * time.Second
		lb = loadbalancers.NewApplicationLoadBalancer(rate,
			c.RequestRateCap)
	case loadbalancers.LoadBalancerTypeNet:
		timeout := time.Duration(c.Timeout) * time.Second
		lb = loadbalancers.NewNetworkLoadBalancer(timeout)
	default:
		return nil, fmt.Errorf("Invalid load balancer type")
	}
	if c.TlsEnabled {
		lb.SetTLS(c.TlsCertFile, c.TlsKeyFile)
	}
	for _, targetGroup := range c.TargetGroups {
		rule := rules.Rule{
			Action:     rules.NewRuleAction(targetGroup.Rule.Action),
			Conditions: targetGroup.Rule.Conditions,
		}
		tg := targets.NewTargetGroup(targetGroup.Name,
			targetGroup.Protocol, rule)
		for _, target := range targetGroup.Targets {
			if target.Url != "" {
				v, err := url.Parse(target.Url)
				if err != nil {
					return nil, err
				}
				tg.AddServiceTarget(v)
			} else {
				tg.AddTarget(target.Host, target.Port)
			}
		}
		if err := lb.AddTargetGroup(tg); err != nil {
			return nil, err
		}
	}
	return lb, nil
}

// run is the main routine that runs the loadbalancer using its given
// configuration file. Returns nil if exited cleanly, otherwise an error is
// returned.
func run(ctx context.Context) error {
	f := ParseFlags()
	c, err := LoadConfig(f.ConfigFile)
	if err != nil {
		return err
	}
	lb, err := newLb(c)
	if err != nil {
		return err
	}
	stopGC := lb.GC()
	defer stopGC()
	stopHealthCheck := lb.HealthCheck(
		time.Duration(c.HealthCheckInterval) * time.Second)
	defer stopHealthCheck()

	laddr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	stopLb, err := lb.Start(laddr, c.Protocol)
	if err != nil {
		return err
	}
	defer stopLb()
	logger.Info(fmt.Sprintf("Listening on %s", laddr))
	<-ctx.Done()
	logger.Info("Received signal, shutting down...")
	return nil
}

func main() {
	ctx := context.Background()
	svc := service.New(ctx)
	if err := svc.Run(run, syscall.SIGINT, syscall.SIGTERM); err != nil {
		fatal("Error: %s", err)
	}
}
