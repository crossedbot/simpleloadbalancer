package networks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
)

func TestGetAttemptsFromContext(t *testing.T) {
	ctx := context.Background()
	actual := getAttemptsFromContext(ctx)
	require.Equal(t, 0, actual)

	expected := 2
	ctx = context.WithValue(ctx, TargetContextAttemptKey, expected)
	actual = getAttemptsFromContext(ctx)
	require.Equal(t, expected, actual)
}

func TestGetRetriesFromContext(t *testing.T) {
	ctx := context.Background()
	actual := getRetriesFromContext(ctx)
	require.Equal(t, 0, actual)

	expected := 2
	ctx = context.WithValue(ctx, TargetContextRetryKey, expected)
	actual = getRetriesFromContext(ctx)
	require.Equal(t, expected, actual)
}

func TestGetTargetProtocol(t *testing.T) {
	protocol := "not-tcp"
	target := targets.NewTarget("", "127.0.0.1", 8080, protocol)
	expected := ""
	actual := getTargetProtocol(target)
	require.Equal(t, expected, actual)

	expected = "tcp"
	target = targets.NewTarget("", "127.0.0.1", 8080, expected)
	actual = getTargetProtocol(target)
	require.Equal(t, expected, actual)

	expected = "udp"
	target = targets.NewTarget("", "127.0.0.1", 8080, expected)
	actual = getTargetProtocol(target)
	require.Equal(t, expected, actual)
}

func TestNetworkPoolAddTarget(t *testing.T) {
	pool := &networkPool{}
	target := targets.NewTarget("", "127.0.0.1", 8080, "tcp")
	pool.AddTarget(target, 0)
	require.Equal(t, 1, len(pool.Targets))
	tgt := pool.Targets[0]
	require.NotNil(t, tgt)
	require.Equal(t, target.Summary(), tgt.Target.Summary())
}

func TestNetworkPoolCurrentTarget(t *testing.T) {
	pool := &networkPool{}
	target := targets.NewTarget("", "127.0.0.1", 8080, "tcp")
	pool.AddTarget(target, 0)
	require.Equal(t, 1, len(pool.Targets))
	tgt := pool.CurrentTarget()
	require.NotNil(t, tgt)
	require.Equal(t, target.Summary(), tgt.Target.Summary())
}

func TestNetworkPoolHealthCheck(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", "{\"hello\": \"world\"}")
		}),
	)
	defer ts.Close()

	pool := &networkPool{}
	targetUrl, err := url.Parse(ts.URL)
	require.Nil(t, err)
	target := targets.NewServiceTarget("", targetUrl)
	pool.AddTarget(target, 3*time.Second)
	require.Equal(t, 1, len(pool.Targets))
	tgt := pool.CurrentTarget()
	require.NotNil(t, tgt)
	interval := 100 * time.Millisecond
	stopHealthCheck := pool.HealthCheck(interval)
	defer stopHealthCheck()

	time.Sleep(interval)
	require.True(t, tgt.Target.IsAlive())
	ts.Close()
	time.Sleep(interval)
	require.False(t, tgt.Target.IsAlive())
}

func TestNetworkPoolLoadBalancer(t *testing.T) {
	body := "{\"hello\": \"world\"}"
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", body)
		}),
	)
	defer ts.Close()

	pool := &networkPool{}
	targetUrl, err := url.Parse(ts.URL)
	require.Nil(t, err)
	target := targets.NewServiceTarget("", targetUrl)
	pool.AddTarget(target, 3*time.Second)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)
	laddr := l.Addr().String()
	require.Nil(t, l.Close())
	stopLb, err := pool.LoadBalancer(laddr, "tcp")
	require.Nil(t, err)
	defer stopLb()

	resp, err := http.Get("http://" + laddr)
	require.Nil(t, err)
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, body, string(respBody))
}

func TestNetworkPoolNextIndex(t *testing.T) {
	pool := &networkPool{}
	target1 := targets.NewTarget("", "127.0.0.1", 8080, "tcp")
	target2 := targets.NewTarget("", "127.0.0.1", 8081, "tcp")
	pool.AddTarget(target1, 0)
	pool.AddTarget(target2, 0)
	expected := 1
	actual := pool.NextIndex()
	require.Equal(t, expected, actual)
}

func TestNetworkPoolNextTarget(t *testing.T) {
	pool := &networkPool{}
	target1 := targets.NewTarget("", "127.0.0.1", 8080, "tcp")
	target2 := targets.NewTarget("", "127.0.0.1", 8081, "tcp")
	pool.AddTarget(target1, 0)
	pool.AddTarget(target2, 0)
	actual := pool.NextTarget()
	require.NotNil(t, actual)
	require.Equal(t, target2.Summary(), actual.Target.Summary())
}

func TestNetworkPoolAttemptNextTarget(t *testing.T) {
	body := "{\"hello\": \"world\"}"
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", body)
		}),
	)
	defer ts.Close()

	pool := &networkPool{}
	targetUrl, err := url.Parse(ts.URL)
	require.Nil(t, err)
	target := targets.NewServiceTarget("", targetUrl)
	pool.AddTarget(target, 3*time.Second)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)
	defer l.Close()
	go func() {
		conn, _ := l.Accept()
		ctx := context.Background()
		pool.AttemptNextTarget(ctx, conn)
	}()

	resp, err := http.Get("http://" + l.Addr().String())
	require.Nil(t, err)
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, body, string(respBody))
}

func TestNetworkPoolHandleConnection(t *testing.T) {
	body := "{\"hello\": \"world\"}"
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", body)
		}),
	)
	defer ts.Close()

	pool := &networkPool{}
	targetUrl, err := url.Parse(ts.URL)
	require.Nil(t, err)
	target := targets.NewServiceTarget("", targetUrl)
	pool.AddTarget(target, 3*time.Second)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)
	defer l.Close()
	go func() {
		conn, _ := l.Accept()
		pool.HandleConnection(conn)
	}()

	resp, err := http.Get("http://" + l.Addr().String())
	require.Nil(t, err)
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, body, string(respBody))
}

func TestNetworkPoolRetryTarget(t *testing.T) {
	body := "{\"hello\": \"world\"}"
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", body)
		}),
	)
	defer ts.Close()

	pool := &networkPool{}
	targetUrl, err := url.Parse(ts.URL)
	require.Nil(t, err)
	target := targets.NewServiceTarget("", targetUrl)
	pool.AddTarget(target, 3*time.Second)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)
	defer l.Close()
	go func() {
		conn, _ := l.Accept()
		ctx := context.Background()
		pool.RetryTarget(ctx, conn)
	}()

	resp, err := http.Get("http://" + l.Addr().String())
	require.Nil(t, err)
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, body, string(respBody))
}
