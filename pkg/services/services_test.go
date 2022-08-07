package services

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/crossedbot/simpleloadbalancer/pkg/ratelimit"
)

func TestAttemptNextService(t *testing.T) {
	rate := time.Second * 3
	capacity := int64(100)
	body := "{\"hello\": \"world\"}"
	errBody := "Service not available\n"
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.Nil(t, err)
	req.Header.Add("X-REAL-IP", "127.0.0.1")

	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", body)
		}),
	)
	defer ts.Close()

	targetUrl, err := url.Parse(ts.URL)
	require.Nil(t, err)
	target := NewTarget("", targetUrl)
	pool := &servicePool{
		RateCapacity: capacity,
		IPRegistry:   ratelimit.NewIPRegistry(time.Duration(rate)),
		Rate:         int64(rate),
	}
	pool.AddService(target)

	// Attempt open service
	rr1 := httptest.NewRecorder()
	pool.attemptNextService(rr1, req)
	resp := rr1.Result()
	respBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, body, string(respBody))

	// Attempt closed service
	ts.Close()
	rr2 := httptest.NewRecorder()
	pool.attemptNextService(rr2, req)
	resp = rr2.Result()
	respBody, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, errBody, string(respBody))
}

func TestGetAttemptsFromContext(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "localhost:8080", nil)
	require.Nil(t, err)

	actual := getAttemptsFromContext(r)
	require.Equal(t, 0, actual)

	expected := 2
	ctx := r.Context()
	ctx = context.WithValue(ctx, ServiceContextAttemptKey, expected)
	actual = getAttemptsFromContext(r.WithContext(ctx))
	require.Equal(t, expected, actual)
}

func TestGetIpFromRequest(t *testing.T) {
	expected := "127.0.0.1"
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.Nil(t, err)

	req.Header.Add("X-REAL-IP", expected)
	actual := getIpFromRequest(req)
	require.Equal(t, expected, actual.String())

	req.Header.Del("X-REAL-IP")
	req.Header.Add("X-FORWARD-FOR", expected)
	actual = getIpFromRequest(req)
	require.Equal(t, expected, actual.String())

	req.Header.Del("X-FORWARD-FOR")
	req.RemoteAddr = net.JoinHostPort(expected, "8080")
	actual = getIpFromRequest(req)
	require.Equal(t, expected, actual.String())
}

func TestGetOrCreateLimiter(t *testing.T) {
	rate := time.Second * 3
	capacity := int64(100)
	pool := &servicePool{
		RateCapacity: capacity,
		IPRegistry:   ratelimit.NewIPRegistry(time.Duration(rate)),
		Rate:         int64(rate),
	}
	ip := net.ParseIP("127.0.0.1")
	require.NotNil(t, ip)
	limiter := pool.IPRegistry.Get(ip)
	require.Nil(t, limiter)
	actual := pool.getOrCreateLimiter(ip)
	require.NotNil(t, actual)
	expected := pool.IPRegistry.Get(ip)
	require.NotNil(t, expected)
	require.Equal(t, expected, actual)
}

func TestGetRetriesFromContext(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "localhost:8080", nil)
	require.Nil(t, err)

	actual := getRetriesFromContext(r)
	require.Equal(t, 0, actual)

	expected := 2
	ctx := r.Context()
	ctx = context.WithValue(ctx, ServiceContextRetryKey, expected)
	actual = getRetriesFromContext(r.WithContext(ctx))
	require.Equal(t, expected, actual)
}

func TestIsServiceAvailable(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", "{\"hello\": \"world\"}")
		}),
	)
	defer ts.Close()

	targetUrl, err := url.Parse(ts.URL)
	require.Nil(t, err)
	target := NewTarget("", targetUrl)
	status := isServiceAvailable(target, 1*time.Second)
	require.True(t, status)

	ts.Close()
	status = isServiceAvailable(target, 1*time.Second)
	require.False(t, status)
}

func TestAddService(t *testing.T) {
	pool := &servicePool{}
	targetUrl, err := url.Parse("localhost:8080")
	require.Nil(t, err)
	target := NewTarget("", targetUrl)
	pool.AddService(target)
	require.Equal(t, 1, len(pool.Services))
	svc := pool.Services[0]
	require.NotNil(t, svc)
	require.Equal(t, target.String(), svc.Target.String())
}

func TestCurrentService(t *testing.T) {
	pool := &servicePool{}
	targetUrl, err := url.Parse("localhost:8080")
	require.Nil(t, err)
	target := NewTarget("", targetUrl)
	pool.AddService(target)
	svc := pool.CurrentService()
	require.NotNil(t, svc)
	require.Equal(t, target.String(), svc.Target.String())
}

func TestHealthCheck(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", "{\"hello\": \"world\"}")
		}),
	)
	defer ts.Close()

	pool := &servicePool{}
	targetUrl, err := url.Parse(ts.URL)
	require.Nil(t, err)
	target := NewTarget("", targetUrl)
	pool.AddService(target)
	svc := pool.CurrentService()
	require.NotNil(t, svc)
	interval := time.Millisecond * 100
	stopHealthCheck := pool.HealthCheck(interval)
	defer stopHealthCheck()

	time.Sleep(interval)
	require.True(t, svc.IsAlive())
	ts.Close()
	time.Sleep(interval)
	require.False(t, svc.IsAlive())
}

func TestLoadBalancer(t *testing.T) {
	rate := time.Second * 3
	capacity := int64(100)
	body := "{\"hello\": \"world\"}"
	errBody := "Service not available\n"
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.Nil(t, err)
	req.Header.Add("X-REAL-IP", "127.0.0.1")

	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", body)
		}),
	)
	defer ts.Close()

	targetUrl, err := url.Parse(ts.URL)
	require.Nil(t, err)
	target := NewTarget("", targetUrl)
	pool := &servicePool{
		RateCapacity: capacity,
		IPRegistry:   ratelimit.NewIPRegistry(time.Duration(rate)),
		Rate:         int64(rate),
	}
	pool.AddService(target)
	fn := pool.LoadBalancer()

	rr1 := httptest.NewRecorder()
	fn(rr1, req)
	resp := rr1.Result()
	respBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, body, string(respBody))

	ts.Close()
	rr2 := httptest.NewRecorder()
	fn(rr2, req)
	resp = rr2.Result()
	respBody, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, errBody, string(respBody))
}

func TestNextIndex(t *testing.T) {
	pool := &servicePool{}
	targetUrl1, err := url.Parse("localhost:8080")
	require.Nil(t, err)
	target1 := NewTarget("", targetUrl1)
	targetUrl2, err := url.Parse("localhost:8081")
	require.Nil(t, err)
	target2 := NewTarget("", targetUrl2)
	pool.AddService(target1)
	pool.AddService(target2)
	expected := 1
	actual := pool.NextIndex()
	require.Equal(t, expected, actual)
}

func TestNextService(t *testing.T) {
	pool := &servicePool{}
	targetUrl1, err := url.Parse("localhost:8080")
	require.Nil(t, err)
	target1 := NewTarget("", targetUrl1)
	targetUrl2, err := url.Parse("localhost:8081")
	require.Nil(t, err)
	target2 := NewTarget("", targetUrl2)
	pool.AddService(target1)
	pool.AddService(target2)
	svc := pool.NextService()
	require.NotNil(t, svc)
	require.Equal(t, svc.Target.String(), target2.String())
}

func TestRetryTargetService(t *testing.T) {
	rate := time.Second * 3
	capacity := int64(100)
	body := "{\"hello\": \"world\"}"
	errBody := "Service not available\n"
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.Nil(t, err)
	req.Header.Add("X-REAL-IP", "127.0.0.1")

	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", body)
		}),
	)
	defer ts.Close()

	targetUrl, err := url.Parse(ts.URL)
	require.Nil(t, err)
	target := NewTarget("", targetUrl)
	pool := &servicePool{
		RateCapacity: capacity,
		IPRegistry:   ratelimit.NewIPRegistry(time.Duration(rate)),
		Rate:         int64(rate),
	}
	pool.AddService(target)

	rr1 := httptest.NewRecorder()
	pool.retryTargetService(rr1, req)
	resp := rr1.Result()
	respBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, body, string(respBody))

	ts.Close()
	rr2 := httptest.NewRecorder()
	pool.retryTargetService(rr2, req)
	resp = rr2.Result()
	respBody, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, errBody, string(respBody))
}

func TestIsAlive(t *testing.T) {
	svc := &service{
		Alive: true,
		Lock:  new(sync.RWMutex),
	}
	require.True(t, svc.IsAlive())
	svc.Alive = false
	require.False(t, svc.IsAlive())
}

func TestSetAlive(t *testing.T) {
	svc := &service{
		Alive: true,
		Lock:  new(sync.RWMutex),
	}
	svc.SetAlive(false)
	require.False(t, svc.IsAlive())
}
