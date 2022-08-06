package ratelimit

import (
	"net"
	"time"

	"github.com/crossedbot/collections/queue"
)

// StopFn is a prototype for a stop routine function.
type StopFn func()

// IPRegistry represents an interface to an IP address registry to map an IP to
// a request rate limiter.
type IPRegistry interface {
	// Get returns the rate limiter for the given IP address.
	Get(ip net.IP) LeakyBucketLimiter

	// Set sets the rate limiter for the given IP address.
	Set(ip net.IP, limiter LeakyBucketLimiter)

	// GC starts a garbage collection routine that can be stopped with the
	// returned stop function.
	GC() StopFn
}

// ipRegistry implements the IPRegistry interface.
type ipRegistry struct {
	Limiters queue.PriorityQueue // The request rate limiters
	Ttl      time.Duration       // Queued request Time-To-Live
}

// NewIPregistry returns a new IPRegistry with given request TTL.
func NewIPRegistry(ttl time.Duration) IPRegistry {
	return &ipRegistry{
		Limiters: queue.NewPriorityQueue(),
		Ttl:      ttl,
	}
}

func (reg *ipRegistry) Get(ip net.IP) LeakyBucketLimiter {
	value := reg.Limiters.Get(ip.String(), reg.Ttl)
	if limiter, ok := value.(LeakyBucketLimiter); ok {
		return limiter
	}
	return nil
}

func (reg *ipRegistry) Set(ip net.IP, limiter LeakyBucketLimiter) {
	reg.Limiters.Add(ip.String(), limiter, reg.Ttl)
}

func (reg *ipRegistry) GC() StopFn {
	quit := make(chan struct{})
	t := time.NewTicker(reg.Ttl)
	go func() {
		for {
			select {
			case <-quit:
				t.Stop()
				return
			case <-t.C:
				reg.Limiters.DeleteExpired(time.Now())
			}
		}
	}()
	return func() { close(quit) }
}
