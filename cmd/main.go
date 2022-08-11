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

	"github.com/crossedbot/common/golang/config"
	"github.com/crossedbot/common/golang/logger"
	"github.com/crossedbot/common/golang/service"

	"github.com/crossedbot/simpleloadbalancer/pkg/loadbalancers"
	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
)

const (
	// Exit codes
	FATAL_EXITCODE = iota + 1
)

// LBTarget is the main configuration for a Load Balancer target. Setting the
// URL will override the Host, Port, and Protocol fields.
type LBTarget struct {
	Name     string `toml:"name"`     // Name of the target
	Host     string `toml:"host"`     // Hostname (IP/Domain/etc)
	Port     int    `toml:"port"`     // Port number of the targeted service
	Protocol string `toml:"protocol"` // TCP, UDP, HTTP, HTTPS, etc.
	Timeout  int    `toml:"timeout"`  // Connection timeout
	Url      string `toml:"url"`      // URL of the targeted service
}

// Config is the main configuration for this application.
type Config struct {
	Type                string     `toml:"type"`
	Host                string     `toml:"host"`
	Port                int        `toml:"port"`
	Protocol            string     `toml:"protocol"`
	RequestRate         int64      `toml:"request_rate"`
	RequestRateCap      int64      `toml:"request_rate_cap"`
	HealthCheckInterval int        `toml:"health_check_interval"`
	Targets             []LBTarget `toml:"targets"`
}

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
		lb = loadbalancers.NewNetworkLoadBalancer()
	default:
		return nil, fmt.Errorf("Invalid load balancer type")
	}
	for _, target := range c.Targets {
		var lbTarget targets.Target
		if target.Url != "" {
			v, err := url.Parse(target.Url)
			if err != nil {
				return nil, err
			}
			lbTarget = targets.NewServiceTarget(target.Name, v)
		} else {
			lbTarget = targets.NewTarget(target.Name, target.Host,
				target.Port, target.Protocol)
		}
		timeout := time.Duration(target.Timeout) * time.Second
		lb.AddTarget(lbTarget, timeout)
	}
	return lb, nil
}

// run is the main routine that runs the loadbalancer using its given
// configuration file. Returns nil if exited cleanly, otherwise an error is
// returned.
func run(ctx context.Context) error {
	f := ParseFlags()
	config.Path(f.ConfigFile)
	var c Config
	if err := config.Load(&c); err != nil {
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
