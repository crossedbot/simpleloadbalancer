package loadbalancers

import (
	"strings"
)

type LoadBalancerType uint32

const (
	LoadBalancerTypeUnknown LoadBalancerType = iota
	LoadBalancerTypeApp
	LoadBalancerTypeNet
)

var LoadBalancerTypeStrings = [][]string{
	[]string{"unknown"},
	[]string{"app", "application"},
	[]string{"net", "network"},
}

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

func (t LoadBalancerType) String() string {
	if t > LoadBalancerType(len(LoadBalancerTypeStrings)) {
		t = LoadBalancerTypeUnknown
	}
	return LoadBalancerTypeStrings[int(t)][0]
}

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
