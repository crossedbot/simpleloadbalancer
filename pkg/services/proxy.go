package services

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/crossedbot/common/golang/logger"
)

type ReverseNetworkProxy interface {
	Proxy(conn net.Conn)
	SetDebug(v bool)
}

type reverseNetworkProxy struct {
	Network string
	Target  string
	Timeout time.Duration
	Debug   bool
}

func NewReverseNetworkProxy(network, target string, to time.Duration) ReverseNetworkProxy {
	return &reverseNetworkProxy{
		Network: network,
		Target:  target,
		Timeout: to,
	}
}

func (p *reverseNetworkProxy) SetDebug(v bool) {
	p.Debug = v
}

func (p *reverseNetworkProxy) Proxy(conn net.Conn) {
	go func() {
		defer conn.Close()
		if p.Debug {
			logger.Info(fmt.Sprintf("Connected: %s", conn.RemoteAddr()))
		}
		remoteConn, err := net.DialTimeout(p.Network, p.Target,
			p.Timeout)
		if err != nil {
			logger.Error(fmt.Sprintf(
				"ReverseNetworkProxy: %s", err))
		}
		defer remoteConn.Close()
		wait := make(chan struct{}, 2)
		go copyConn(wait, conn, remoteConn, p.Debug)
		go copyConn(wait, remoteConn, conn, p.Debug)
		<-wait
		if p.Debug {
			logger.Info(fmt.Sprintf("Closed: %s", conn.RemoteAddr()))
		}
	}()
}

func copyConn(closer chan struct{}, src io.Reader, dst io.Writer, debug bool) {
	if debug {
		_, _ = io.Copy(os.Stdout, io.TeeReader(src, dst))
	} else {
		_, _ = io.Copy(dst, src)
	}
	closer <- struct{}{}
}
