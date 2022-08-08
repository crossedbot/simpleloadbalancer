package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/crossedbot/common/golang/config"
	"github.com/crossedbot/common/golang/logger"

	"github.com/crossedbot/simpleloadbalancer/pkg/services"
	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
)

const (
	FATAL_EXITCODE = iota + 1
)

type Config struct {
	Host                string   `toml:"host"`
	Port                int      `toml:"port"`
	RequestRate         int64    `toml:"request_rate"`
	RequestRateCap      int64    `toml:"request_rate_cap"`
	HealthCheckInterval int      `toml:"health_check_interval"`
	Targets             []string `toml:"targets"`
}

func fatal(format string, a ...interface{}) {
	logger.Error(fmt.Errorf(format, a...))
	os.Exit(FATAL_EXITCODE)
}

func newServicePool(c Config) services.ServicePool {
	rate := time.Duration(c.RequestRate) * time.Second
	pool := services.New(int64(rate), c.RequestRateCap)
	for _, target := range c.Targets {
		v, err := url.Parse(target)
		if err == nil {
			pool.AddService(targets.NewServiceTarget("", v))
		}
	}
	return pool
}

func run(ctx context.Context) error {
	f := ParseFlags()
	config.Path(f.ConfigFile)
	var c Config
	if err := config.Load(&c); err != nil {
		return err
	}
	pool := newServicePool(c)
	stopGC := pool.GC()
	defer stopGC()
	stopHealthCheck := pool.HealthCheck(
		time.Duration(c.HealthCheckInterval) * time.Second)
	defer stopHealthCheck()

	hostPort := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	server := http.Server{
		Addr:    hostPort,
		Handler: http.HandlerFunc(pool.LoadBalancer()),
	}

	logger.Info(fmt.Sprintf("Load balancer started on '%s'", hostPort))
	if err := server.ListenAndServe(); err != nil {
		fatal("Error: %s", err)
	}
	return nil
}

func main() {
	ctx := context.Background()
	run(ctx)
}
