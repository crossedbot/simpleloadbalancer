package networks

import (
	"sync"

	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
)

type networkTarget struct {
	Target *targets.Target
	Alive  bool
	Proxy  ReverseNetworkProxy
	Lock   *sync.RWMutex
}
