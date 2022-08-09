package networks

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/crossedbot/common/golang/logger"
)

type ErrorHandlerFunc func(context.Context, net.Conn, error)

type ReverseNetworkProxy interface {
	Proxy(ctx context.Context, conn net.Conn)
	SetDebug(v bool)
	SetErrorHandler(fn ErrorHandlerFunc)
}

type reverseNetworkProxy struct {
	HandleError ErrorHandlerFunc
	Network     string
	Target      string
	Timeout     time.Duration
	Debug       bool
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

func (p *reverseNetworkProxy) SetErrorHandler(fn ErrorHandlerFunc) {
	p.HandleError = fn
}

func (p *reverseNetworkProxy) Proxy(ctx context.Context, conn net.Conn) {
	go func() {
		if p.Debug {
			logger.Info(fmt.Sprintf(
				"Connected: %s", conn.RemoteAddr()))
		}
		remoteConn, err := net.DialTimeout(p.Network, p.Target,
			p.Timeout)
		if err != nil {
			p.HandleError(ctx, conn, err)
			return
		}
		defer remoteConn.Close()
		defer conn.Close()
		wait := make(chan struct{}, 2)
		go copyConn(wait, conn, remoteConn, p.Debug)
		go copyConn(wait, remoteConn, conn, p.Debug)
		<-wait
		if p.Debug {
			logger.Info(fmt.Sprintf(
				"Closed: %s", conn.RemoteAddr()))
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
