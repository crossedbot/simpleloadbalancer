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

func TestIsTLS(t *testing.T) {
	tests := []struct {
		Proto    string
		Expected bool
	}{
		{"https", true},
		{"http", false},
		{"LDAPS", true},
		{"LDAP", false},
		{"wat", false},
	}
	for _, test := range tests {
		require.Equal(t, test.Expected, IsTLS(test.Proto))
	}
}

func TestTargetErrResponseFormat(t *testing.T) {
	expected := ResponseFormatJson
	tgt := &target{ErrRespFmt: expected}
	actual := tgt.ErrResponseFormat()
	require.Equal(t, expected, actual)
}

func TestTargetGet(t *testing.T) {
	host := "example.com"
	port := "8080"
	proto := "http"
	respFmt := "json"
	targetUrl, err := url.Parse(
		fmt.Sprintf("%s://%s:%s", proto, host, port))
	require.Nil(t, err)
	target := NewServiceTarget(targetUrl)
	require.NotNil(t, target)
	target.SetErrResponseFormat(ResponseFormatJson)
	require.Equal(t, "true", target.Get("alive"))
	require.Equal(t, host, target.Get("host"))
	require.Equal(t, port, target.Get("port"))
	require.Equal(t, proto, target.Get("protocol"))
	require.Equal(t, respFmt, target.Get("response_format"))
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

func TestTargetSetErrResponseFormat(t *testing.T) {
	expected := ResponseFormatJson
	tgt := &target{}
	tgt.SetErrResponseFormat(expected)
	actual := tgt.ErrResponseFormat()
	require.Equal(t, expected, actual)
}

func TestTargetSummary(t *testing.T) {
	host := "example.com"
	port := 8080
	proto := "http"
	expected := fmt.Sprintf(
		"alive=true,host=%s,port=%d,protocol=%s,response_format=%s,type=%s",
		host, port, proto,
		ResponseFormatPlain.String(),
		TargetTypeDomain.String(),
	)
	targetUrl, err := url.Parse(
		fmt.Sprintf("%s://%s:%d", proto, host, port))
	require.Nil(t, err)
	target := NewServiceTarget(targetUrl)
	require.NotNil(t, target)
	summary := target.Summary()
	require.Equal(t, expected, summary)
}

func TestTargetURL(t *testing.T) {
	tests := []struct {
		Host     string
		Port     int
		Protocol string
		Expected string
	}{
		{
			Host:     "example.com",
			Port:     0,
			Protocol: "https",
			Expected: "https://example.com",
		}, {
			Host:     "127.0.0.1",
			Port:     8080,
			Protocol: "http",
			Expected: "http://127.0.0.1:8080",
		}, {
			Host:     "10.125.16.2",
			Port:     0,
			Protocol: "ssh",
			Expected: "ssh://10.125.16.2",
		},
	}
	for _, test := range tests {
		tgt := &target{
			Host:     test.Host,
			Port:     test.Port,
			Protocol: test.Protocol,
		}
		require.Equal(t, test.Expected, tgt.URL())
	}
}
