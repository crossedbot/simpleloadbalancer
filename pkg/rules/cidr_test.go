package rules

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewIPAddr(t *testing.T) {
	tests := []struct {
		IP       string
		Expected []uint32
	}{
		{"127.126.125.124", []uint32{2138996092}},
		{"2a02:ff0:2f9:b707::1", []uint32{704778224, 49919751, 0, 1}},
	}
	for _, test := range tests {
		actual := NewIPAddr(net.ParseIP(test.IP))
		require.Equal(t, test.Expected, []uint32(actual))
	}
}

func TestNetworkContains(t *testing.T) {
	tests := []struct {
		Network  string
		IP       string
		Expected bool
	}{
		{"192.128.0.0/24", "192.128.2.10", false},
		{"192.128.0.0/24", "192.128.0.0", true},
		{"192.128.0.0/24", "192.128.0.255", true},
		{"2a02::0/120", "2a02:ff0:2f9:b707::1", false},
		{"2a02::0/120", "2a02::0", true},
		{"2a02::0/120", "2a02::ff", true},
	}
	for _, test := range tests {
		_, n, err := net.ParseCIDR(test.Network)
		require.Nil(t, err)
		ip := net.ParseIP(test.IP)
		require.Equal(t, test.Expected, NetworkContains(*n, ip))
	}
}

func TestIsCIDR(t *testing.T) {
	tests := []struct {
		Network  string
		Expected bool
	}{
		{"192.128.0.0", false},
		{"192.128.0.0/24", true},
		{"2a02::0", false},
		{"2a02::0/120", true},
	}
	for _, test := range tests {
		require.Equal(t, test.Expected, IsCIDR(test.Network))
	}
}
