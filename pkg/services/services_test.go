package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/crossedbot/simpleloadbalancer/pkg/ratelimit"
	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
	"github.com/crossedbot/simpleloadbalancer/pkg/templates"
)

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

func TestHandleServiceUnavailable(t *testing.T) {
	rr1 := httptest.NewRecorder()
	errFmt := ResponseFormatHtml
	expected := templates.ServiceUnavailablePage()
	handleServiceUnavailable(rr1, errFmt)
	resp := rr1.Result()
	actual, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, expected, string(actual))

	expected = "Service not available\n"
	rr2 := httptest.NewRecorder()
	errFmt = ResponseFormatJson
	b, err := json.Marshal(ResponseError{
		Code:    http.StatusServiceUnavailable,
		Message: expected[:len(expected)-1],
	})
	require.Nil(t, err)
	handleServiceUnavailable(rr2, errFmt)
	resp = rr2.Result()
	actual, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, b, actual)

	rr3 := httptest.NewRecorder()
	errFmt = ResponseFormatPlain
	handleServiceUnavailable(rr3, errFmt)
	resp = rr3.Result()
	actual, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, expected, string(actual))

	rr4 := httptest.NewRecorder()
	errFmt = ResponseFormatUnknown
	handleServiceUnavailable(rr4, errFmt)
	resp = rr4.Result()
	actual, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, expected, string(actual))
}

func TestHandleTooManyRequests(t *testing.T) {
	to := 10

	rr1 := httptest.NewRecorder()
	errFmt := ResponseFormatHtml
	expected := templates.TooManyRequestsPage(to)
	handleTooManyRequests(rr1, errFmt, time.Duration(to)*time.Second)
	resp := rr1.Result()
	actual, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	require.Equal(t, expected, string(actual))

	expected = fmt.Sprintf("Too many requests - try again in %d seconds\n",
		to)
	rr2 := httptest.NewRecorder()
	errFmt = ResponseFormatJson
	b, err := json.Marshal(ResponseError{
		Code:    http.StatusTooManyRequests,
		Message: expected[:len(expected)-1],
	})
	require.Nil(t, err)
	handleTooManyRequests(rr2, errFmt, time.Duration(to)*time.Second)
	resp = rr2.Result()
	actual, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	require.Equal(t, b, actual)

	rr3 := httptest.NewRecorder()
	errFmt = ResponseFormatPlain
	handleTooManyRequests(rr3, errFmt, time.Duration(to)*time.Second)
	resp = rr3.Result()
	actual, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	require.Equal(t, expected, string(actual))

	rr4 := httptest.NewRecorder()
	errFmt = ResponseFormatUnknown
	handleTooManyRequests(rr4, errFmt, time.Duration(to)*time.Second)
	resp = rr4.Result()
	actual, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	require.Equal(t, expected, string(actual))
}

func TestServicePoolAddService(t *testing.T) {
	pool := &servicePool{}
	targetUrl, err := url.Parse("localhost:8080")
	require.Nil(t, err)
	target := targets.NewServiceTarget(targetUrl)
	pool.AddService(target)
	require.Equal(t, 1, len(pool.Services))
	svc := pool.Services[0]
	require.NotNil(t, svc)
	require.Equal(t, target.Summary(), svc.Target.Summary())
}

func TestServicePoolAttemptNextService(t *testing.T) {
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
	target := targets.NewServiceTarget(targetUrl)
	pool := &servicePool{
		RateCapacity: capacity,
		IPRegistry:   ratelimit.NewIPRegistry(time.Duration(rate)),
		Rate:         int64(rate),
	}
	pool.AddService(target)

	// Attempt open service
	rr1 := httptest.NewRecorder()
	pool.AttemptNextService(rr1, req)
	resp := rr1.Result()
	respBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, body, string(respBody))

	// Attempt closed service
	ts.Close()
	rr2 := httptest.NewRecorder()
	pool.AttemptNextService(rr2, req)
	resp = rr2.Result()
	respBody, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, errBody, string(respBody))
}

func TestServicePoolCurrentService(t *testing.T) {
	pool := &servicePool{}
	targetUrl, err := url.Parse("localhost:8080")
	require.Nil(t, err)
	target := targets.NewServiceTarget(targetUrl)
	pool.AddService(target)
	svc := pool.CurrentService()
	require.NotNil(t, svc)
	require.Equal(t, target.Summary(), svc.Target.Summary())
}

func TestServicePoolGetOrCreateLimiter(t *testing.T) {
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
	actual := pool.GetOrCreateLimiter(ip)
	require.NotNil(t, actual)
	expected := pool.IPRegistry.Get(ip)
	require.NotNil(t, expected)
	require.Equal(t, expected, actual)
}

func TestServicePoolHealthCheck(t *testing.T) {
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
	target := targets.NewServiceTarget(targetUrl)
	pool.AddService(target)
	svc := pool.CurrentService()
	require.NotNil(t, svc)
	interval := time.Millisecond * 100
	stopHealthCheck := pool.HealthCheck(interval)
	defer stopHealthCheck()

	time.Sleep(interval)
	require.True(t, svc.Target.IsAlive())
	ts.Close()
	time.Sleep(interval)
	require.False(t, svc.Target.IsAlive())
}

func TestServicePoolLoadBalancer(t *testing.T) {
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
	target := targets.NewServiceTarget(targetUrl)
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

func TestServiceSetResponseFormat(t *testing.T) {
	expected := ResponseFormatJson
	pool := &servicePool{}
	pool.SetResponseFormat(expected)
	require.Equal(t, expected, pool.RespFormat)
}

func TestServicePoolNextIndex(t *testing.T) {
	pool := &servicePool{}
	targetUrl1, err := url.Parse("localhost:8080")
	require.Nil(t, err)
	target1 := targets.NewServiceTarget(targetUrl1)
	targetUrl2, err := url.Parse("localhost:8081")
	require.Nil(t, err)
	target2 := targets.NewServiceTarget(targetUrl2)
	pool.AddService(target1)
	pool.AddService(target2)
	expected := 1
	actual := pool.NextIndex()
	require.Equal(t, expected, actual)
}

func TestServicePoolNextService(t *testing.T) {
	pool := &servicePool{}
	targetUrl1, err := url.Parse("localhost:8080")
	require.Nil(t, err)
	target1 := targets.NewServiceTarget(targetUrl1)
	targetUrl2, err := url.Parse("localhost:8081")
	require.Nil(t, err)
	target2 := targets.NewServiceTarget(targetUrl2)
	pool.AddService(target1)
	pool.AddService(target2)
	svc := pool.NextService()
	require.NotNil(t, svc)
	require.Equal(t, svc.Target.Summary(), target2.Summary())
}

func TestServicePoolRetryService(t *testing.T) {
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
	target := targets.NewServiceTarget(targetUrl)
	pool := &servicePool{
		RateCapacity: capacity,
		IPRegistry:   ratelimit.NewIPRegistry(time.Duration(rate)),
		Rate:         int64(rate),
	}
	pool.AddService(target)

	rr1 := httptest.NewRecorder()
	pool.RetryService(rr1, req)
	resp := rr1.Result()
	respBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, body, string(respBody))

	ts.Close()
	rr2 := httptest.NewRecorder()
	pool.RetryService(rr2, req)
	resp = rr2.Result()
	respBody, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, errBody, string(respBody))
}
