package loadbalancers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestType(t *testing.T) {
	expected := LoadBalancerTypeUnknown
	actual := Type("idontexist")
	require.Equal(t, expected, actual)
	actual = Type("unknown")
	require.Equal(t, expected, actual)

	expected = LoadBalancerTypeApp
	actual = Type("app")
	require.Equal(t, expected, actual)
	actual = Type("application")
	require.Equal(t, expected, actual)
	actual = Type("aPpLiCaTiOn")
	require.Equal(t, expected, actual)

	expected = LoadBalancerTypeNet
	actual = Type("net")
	require.Equal(t, expected, actual)
	actual = Type("network")
	require.Equal(t, expected, actual)
	actual = Type("nEtWoRk")
	require.Equal(t, expected, actual)
}

func TestLoadBalancerTypeString(t *testing.T) {
	for idx, expected := range LoadBalancerTypeStrings {
		lbType := LoadBalancerType(idx)
		require.Greater(t, len(expected), 0)
		require.Equal(t, expected[0], lbType.String())
	}
}

func TestLoadBalancerTypeLong(t *testing.T) {
	for idx, expected := range LoadBalancerTypeStrings {
		lbType := LoadBalancerType(idx)
		length := len(expected)
		require.Greater(t, length, 0)
		if length > 1 {
			require.Equal(t, expected[1], lbType.Long())
			continue
		}
		require.Equal(t, expected[0], lbType.Long())
	}
}
