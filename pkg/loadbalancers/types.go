package loadbalancers

import (
	"strings"
)

// LoadBalancerType is a numerical representation of a load balancer type.
type LoadBalancerType uint32

const (
	// LB types
	LoadBalancerTypeUnknown LoadBalancerType = iota
	LoadBalancerTypeApp
	LoadBalancerTypeNet
)

// LoadBalancerTypeStrings is a list of names (short and long) for LB types. If
// the LB type has a long name, it will always follow the short name.
var LoadBalancerTypeStrings = [][]string{
	[]string{"unknown"},
	[]string{"app", "application"},
	[]string{"net", "network"},
}

// Type returns the LoadBalancerType for a given string. If the string is not
// recognized, LoadBalancerTypeUnknown is returned.
func Type(v string) LoadBalancerType {
	for idx, ss := range LoadBalancerTypeStrings {
		for _, s := range ss {
			if strings.EqualFold(s, v) {
				return LoadBalancerType(idx)
			}
		}
	}
	return LoadBalancerTypeUnknown
}

// String returns a string representation of the LoadBalancerType.
func (t LoadBalancerType) String() string {
	if t > LoadBalancerType(len(LoadBalancerTypeStrings)) {
		t = LoadBalancerTypeUnknown
	}
	return LoadBalancerTypeStrings[int(t)][0]
}

// Long returns the long name for a LoadBalancerType; if it exists.
func (t LoadBalancerType) Long() string {
	if t > LoadBalancerType(len(LoadBalancerTypeStrings)) {
		t = LoadBalancerTypeUnknown
	}
	ss := LoadBalancerTypeStrings[int(t)]
	idx := 0
	if len(ss) > 1 {
		idx = 1
	}
	return LoadBalancerTypeStrings[int(t)][idx]
}
