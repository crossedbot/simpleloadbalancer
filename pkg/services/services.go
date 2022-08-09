package services

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/crossedbot/common/golang/logger"

	"github.com/crossedbot/simpleloadbalancer/pkg/ratelimit"
	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
)

const (
	// Service constants
	ServiceMaxAttempts   = 3
	ServiceMaxRetries    = 3
	ServiceRetryInterval = time.Millisecond * 100

	// Context keys
	ServiceContextAttemptKey = iota + 1
	ServiceContextRetryKey
)

// StopFn is a prototype for a stop routine function.
type StopFn func()

// service represents a HTTP service.
type service struct {
	Target targets.Target         // Target service URL
	Proxy  *httputil.ReverseProxy // Proxy to forward requests
}

// ServicePool represents a pool of services for tracking and balancing requests
// on behalf of clients to the backend services.
type ServicePool interface {
	// AddService adds a new service to the pool for the given target URL.
	AddService(target targets.Target) error

	// GC starts the IP registry garbage collector and returns a stop
	// function to exit garbage collection loop; effectively stopping the
	// routine.
	GC() StopFn

	// HealthCheck starts a routine to passively track the health of the
	// targeted services. It returns a function that can be called to stop
	// the health checking routine.
	HealthCheck(interval time.Duration) StopFn

	// LoadBalancer returns a handler func that will balance requests across
	// the targeted services using the Round Robin strategy. Further,
	// requests are rate limited by IP address.
	LoadBalancer() http.HandlerFunc
}

// servicePool implements a ServicePool to track and balance client requests to
// backend services.
type servicePool struct {
	Index        uint64               // Current service index
	IPRegistry   ratelimit.IPRegistry // IP registry for rate limiting
	Rate         int64                // Request rate in Nanoseconds
	RateCapacity int64                // Capacity of requests in a queue
	Services     []*service           // List of backend services
}

func New(rate int64, rateCap int64) ServicePool {
	return &servicePool{
		IPRegistry:   ratelimit.NewIPRegistry(time.Duration(rate)),
		Rate:         rate,
		RateCapacity: rateCap,
	}
}

func (pool *servicePool) AddService(target targets.Target) error {
	proto := target.Get("protocol")
	host := target.Get("host")
	if port := target.Get("port"); port != "" {
		host = net.JoinHostPort(host, port)
	}
	urlStr := fmt.Sprintf("%s://%s", proto, host)
	targetUrl, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	svc := &service{
		Target: target,
		Proxy:  httputil.NewSingleHostReverseProxy(targetUrl),
	}
	svc.Proxy.ErrorHandler =
		func(w http.ResponseWriter, r *http.Request, err error) {
			// Handle service failures by retrying the service, if
			// that fails attempt another service.
			alive := pool.retryTargetService(w, r)
			svc.Target.SetAlive(alive)
			if !alive && !pool.attemptNextService(w, r) {
				http.Error(w, "Service not available",
					http.StatusServiceUnavailable)
			}
		}
	pool.Services = append(pool.Services, svc)
	return nil
}

func (pool *servicePool) CurrentService() *service {
	idx := int(pool.Index) % len(pool.Services)
	return pool.Services[idx]
}

func (pool *servicePool) GC() StopFn {
	return StopFn(pool.IPRegistry.GC())
}

func (pool *servicePool) HealthCheck(interval time.Duration) StopFn {
	quit := make(chan struct{})
	t := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-quit:
				t.Stop()
				return
			case <-t.C:
				for _, svc := range pool.Services {
					alive := svc.Target.IsAvailable(
						time.Second * 3)
					svc.Target.SetAlive(alive)
				}
			}
		}
	}()
	return func() { close(quit) }
}

func (pool *servicePool) LoadBalancer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer prExTim(r.URL.RequestURI())()

		ip := getIpFromRequest(r)
		if ip == nil {
			// Just return because it doesn't know who you are
			logger.Info("Failed to parse IP address")
			return
		}
		// Retrieve or create the rate limiter for the extracted IP and
		// check if it has reached its request capacity.
		limiter := pool.getOrCreateLimiter(ip)
		next, err := limiter.Next()
		if err == ratelimit.ErrLimiterMaxCapacity {
			msg := fmt.Sprintf(
				"Too many requests - try again in %d secs",
				int(next.Seconds()),
			)
			http.Error(w, msg, http.StatusTooManyRequests)
			return
		}
		// Service the request
		if !pool.attemptNextService(w, r) {
			http.Error(w, "Service not available",
				http.StatusServiceUnavailable)
			return
		}
	}
}

func (pool *servicePool) NextIndex() int {
	return int(atomic.AddUint64(&pool.Index, uint64(1)) %
		uint64(len(pool.Services)))
}

func (pool *servicePool) NextService() *service {
	next := pool.NextIndex()
	cycle := len(pool.Services) + next
	for i := next; i < cycle; i++ {
		idx := i % len(pool.Services)
		if pool.Services[idx].Target.IsAlive() {
			if i != next {
				atomic.StoreUint64(&pool.Index, uint64(idx))
			}
			return pool.Services[idx]
		}
	}
	return nil
}

// attemptNextService attempts the next service at pool.Index + 1 and tracks the
// attempts in the request's context. If the attempts exceed the maximum number
// of service attempts, the request is canceled. Returns true if attempt is
// made, otherwise false returns indicating the request was canceled.
func (pool *servicePool) attemptNextService(w http.ResponseWriter, r *http.Request) bool {
	attempts := getAttemptsFromContext(r)
	if attempts < ServiceMaxAttempts {
		svc := pool.NextService()
		if svc != nil {
			ctx := context.WithValue(r.Context(),
				ServiceContextAttemptKey, attempts+1)
			svc.Proxy.ServeHTTP(w, r.WithContext(ctx))
			return true
		}
	}
	return false
}

// getOrCreateLimiter returns the rate limiter for a given IP address. If a rate
// limiter does not exist yet for the IP address, a new one is created and
// returned.
func (pool *servicePool) getOrCreateLimiter(ip net.IP) ratelimit.LeakyBucketLimiter {
	limiter := pool.IPRegistry.Get(ip)
	if limiter == nil {
		limiter = ratelimit.NewLeakyBucket(pool.RateCapacity, pool.Rate)
		pool.IPRegistry.Set(ip, limiter)
	}
	return limiter
}

// retryTargetService retries the current service at a set interval and tracks
// the number of retries attempted in the request's context. If the number
// retries exceed the maxmimum number of retries, the request is canceled for
// the current service backend. Returns true if a retry was attempted, otherwise
// false is returned to indicate the request was canceled.
func (pool *servicePool) retryTargetService(w http.ResponseWriter, r *http.Request) bool {
	retries := getRetriesFromContext(r)
	after := time.After(ServiceRetryInterval)
	for retries < ServiceMaxRetries {
		select {
		case <-after:
			svc := pool.CurrentService()
			if svc == nil {
				return false
			}
			ctx := context.WithValue(r.Context(),
				ServiceContextRetryKey, retries+1)
			svc.Proxy.ServeHTTP(w, r.WithContext(ctx))
			return true
		}
	}
	return false
}

// getAttemptsFromContext returns the number of attempts tracked in the given
// request.
func getAttemptsFromContext(r *http.Request) int {
	attempts, ok := r.Context().Value(ServiceContextAttemptKey).(int)
	if ok {
		return attempts
	}
	return 0
}

// getIpFromRequest returns the IP address of the client from given request. If
// an IP address could not be extracted, nil is returned instead. It first tries
// the "X-REAL-IP" header, then the "X-FORWARD_FOR" header, and then finally
// tries to extract the IP from the request's remote address field.
func getIpFromRequest(r *http.Request) net.IP {
	v := r.Header.Get("X-REAL-IP")
	if ip := net.ParseIP(v); ip != nil {
		return ip
	}
	v = r.Header.Get("X-FORWARD-FOR")
	parts := strings.Split(v, ",")
	for _, p := range parts {
		if ip := net.ParseIP(p); ip != nil {
			return ip
		}
	}
	v, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if ip := net.ParseIP(v); ip != nil {
			return ip
		}
	}
	return nil
}

// getRetriesFromContext returns the number of retries tracked in the given
// request.
func getRetriesFromContext(r *http.Request) int {
	retries, ok := r.Context().Value(ServiceContextRetryKey).(int)
	if ok {
		return retries
	}
	return 0
}

// prExTim logs the execution time for a given routine name.
func prExTim(name string) func() {
	now := time.Now()
	return func() {
		logger.Info(fmt.Sprintf("%s took %s", name, time.Since(now)))
	}
}
