package ratelimit

import (
	"net"
	"testing"
	"time"

	"github.com/crossedbot/collections/queue"
	"github.com/stretchr/testify/require"
)

func TestIpRegistryGet(t *testing.T) {
	ttl := time.Second * 3
	ip := net.ParseIP("127.0.0.1")
	require.NotNil(t, ip)
	limiter := NewLeakyBucket(int64(3), int64(ttl))
	reg := &ipRegistry{Limiters: queue.NewPriorityQueue()}
	reg.Limiters.Add(ip.String(), limiter, ttl)
	actual := reg.Get(ip)
	require.Equal(t, limiter, actual)
}

func TestIpRegistrySet(t *testing.T) {
	ttl := time.Second * 3
	ip := net.ParseIP("127.0.0.1")
	require.NotNil(t, ip)
	limiter := NewLeakyBucket(int64(3), int64(ttl))
	reg := &ipRegistry{
		Limiters: queue.NewPriorityQueue(),
		Ttl:      ttl,
	}
	reg.Set(ip, limiter)
	actual := reg.Limiters.Get(ip.String(), ttl)
	require.Equal(t, limiter, actual)
}

func TestIpRegistryGC(t *testing.T) {
	ttl := time.Millisecond * 100
	ip := net.ParseIP("127.0.0.1")
	require.NotNil(t, ip)
	reg := &ipRegistry{
		Limiters: queue.NewPriorityQueue(),
		Ttl:      ttl,
	}
	stopFn := reg.GC()
	defer stopFn()
	limiter := NewLeakyBucket(int64(3), int64(ttl))
	reg.Set(ip, limiter)
	exists := reg.Get(ip)
	require.NotNil(t, exists)
	time.Sleep(ttl + (time.Millisecond * 10))
	exists = reg.Get(ip)
	require.Nil(t, exists)
}
