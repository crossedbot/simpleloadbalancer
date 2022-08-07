package services

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

/*
   "http":   80,
   "ssh":    22,
   "telnet": 23,
   "smtp":   25,
   "dns":    53,
   "ntp":    123,
   "ldap":   389,
   "https":  443,
   "ldaps":  636,
*/

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

func TestTargetURL(t *testing.T) {
	expected, err := url.Parse("http://example.com:8080")
	require.Nil(t, err)
	target := NewTarget("", expected)
	require.NotNil(t, target)
	actual, err := target.URL()
	require.Nil(t, err)
	require.Equal(t, expected.String(), actual.String())
}

func TestTargetString(t *testing.T) {
	expected := "http://example.com:8080"
	targetUrl, err := url.Parse(expected)
	require.Nil(t, err)
	target := NewTarget("", targetUrl)
	require.NotNil(t, target)
	actual := target.String()
	require.Equal(t, expected, actual)

}
