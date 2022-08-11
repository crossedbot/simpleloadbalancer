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

// ErrorHandlerFunc is a prototype for network proxy error handler.
type ErrorHandlerFunc func(context.Context, net.Conn, error)

// ReverseNetworkProxy represents an interface to a network-level reverse proxy
// to forward TCP, UDP, etc. connections.
type ReverseNetworkProxy interface {
	// Proxy forwards the given connection to the targeted service.
	Proxy(ctx context.Context, conn net.Conn)

	// SetDebug sets the debugging attribute to print things like the
	// forwarded/reversed packets during the lifetime of the connection.
	SetDebug(v bool)

	// SetErrorHandler sets the proxy's error handler. For example, when
	// connecting to the target service fails, an error handler may be
	// useful for retrying the connection.
	SetErrorHandler(fn ErrorHandlerFunc)
}

// reverseNetworkProxy implements the ReverseNetworkProxy and manages target and
// connection related attributes.
type reverseNetworkProxy struct {
	HandleError ErrorHandlerFunc
	Network     string
	Target      string
	Timeout     time.Duration
	Debug       bool
}

// NewReverseNetworkProxy returns a new network proxy that targets the given
// host (target) stirng for a given network protocol and connection timeout.
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
		_, cancelCtx := context.WithCancel(ctx)
		defer cancelCtx()
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
