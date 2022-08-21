package targets

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetPort(t *testing.T) {
	for proto, expected := range ProtocolPorts {
		actual := GetPort(proto)
		require.Equal(t, expected, actual)
	}
}

func TestGetProtocol(t *testing.T) {
	for expected, port := range ProtocolPorts {
		actual := GetProtocol(port)
		require.Equal(t, expected, actual)
	}
}

func TestGetTransport(t *testing.T) {
	for proto, expected := range ProtocolTransports {
		actual := GetTransport(proto)
		require.Equal(t, expected, actual)
	}
}

func TestTargetGet(t *testing.T) {
	host := "example.com"
	port := "8080"
	proto := "http"
	targetUrl, err := url.Parse(
		fmt.Sprintf("%s://%s:%s", proto, host, port))
	require.Nil(t, err)
	target := NewServiceTarget(targetUrl)
	require.NotNil(t, target)
	require.Equal(t, "true", target.Get("alive"))
	require.Equal(t, host, target.Get("host"))
	require.Equal(t, port, target.Get("port"))
	require.Equal(t, proto, target.Get("protocol"))
	require.Equal(t, TargetTypeDomain.String(), target.Get("type"))
}

func TestTargetIsAlive(t *testing.T) {
	target := &target{
		Alive: true,
		Lock:  new(sync.RWMutex),
	}
	require.True(t, target.IsAlive())
	target.Alive = false
	require.False(t, target.IsAlive())
}

func TestTargetIsAvailable(t *testing.T) {
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
	target := NewServiceTarget(targetUrl)
	status := target.IsAvailable(1 * time.Second)
	require.True(t, status)

	ts.Close()
	status = target.IsAvailable(1 * time.Second)
	require.False(t, status)
}

func TestTargetSetAlive(t *testing.T) {
	target := &target{
		Alive: true,
		Lock:  new(sync.RWMutex),
	}
	target.SetAlive(false)
	require.False(t, target.IsAlive())
}

func TestTargetSummary(t *testing.T) {
	host := "example.com"
	port := 8080
	proto := "http"
	expected := fmt.Sprintf(
		"alive=true,host=%s,port=%d,protocol=%s,type=%s",
		host, port, proto, TargetTypeDomain.String())
	targetUrl, err := url.Parse(
		fmt.Sprintf("%s://%s:%d", proto, host, port))
	require.Nil(t, err)
	target := NewServiceTarget(targetUrl)
	require.NotNil(t, target)
	summary := target.Summary()
	require.Equal(t, expected, summary)
}
