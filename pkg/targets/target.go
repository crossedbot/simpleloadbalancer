package targets

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// Protocol and port maps
	ProtocolPorts = map[string]int{
		"http":   80,
		"ssh":    22,
		"telnet": 23,
		"smtp":   25,
		"dns":    53,
		"ntp":    123,
		"ldap":   389,
		"https":  443,
		"ldaps":  636,
	}
	ProtocolTransports = map[string][]string{
		"tcp":    []string{"tcp"},
		"udp":    []string{"udp"},
		"http":   []string{"tcp"},
		"telnet": []string{"tcp"},
		"smtp":   []string{"tcp"},
		"dns":    []string{"udp", "tcp"},
		"ntp":    []string{"udp"},
		"ldap":   []string{"tcp"},
		"https":  []string{"tcp"},
		"ldaps":  []string{"tcp"},
	}

	// Errors
	ErrMissingProtocol = errors.New("Target is missing protocol")
	ErrUnknownProtocol = errors.New("Unknown network protocol")
)

// GetPort returns the common port number for the given application protocol
// string.
func GetPort(protocol string) int {
	return ProtocolPorts[strings.ToLower(protocol)]
}

// GetProtocol returns the common application protocol string for the given port
// number.
func GetProtocol(port int) string {
	for proto, curr := range ProtocolPorts {
		if port == curr {
			return proto
		}
	}
	return ""
}

// GetTransport returns the common transport protocols for the given application
// protocol string.
func GetTransport(protocol string) []string {
	return ProtocolTransports[strings.ToLower(protocol)]
}

// IsTLS returns true if the given protocol uses SSL/TLS; I.E. HTTPS.
func IsTLS(protocol string) bool {
	isTls := false
	switch strings.ToLower(protocol) {
	case "https", "ldaps":
		isTls = true
	}
	return isTls
}

// TargetType represents a type of load balancer target type.
type TargetType uint32

const (
	// Target types
	TargetTypeIP TargetType = iota + 1
	TargetTypeDomain
)

// String returns the string representation of the target type.
func (tt TargetType) String() string {
	s := ""
	switch tt {
	case TargetTypeIP:
		s = "ip"
	case TargetTypeDomain:
		s = "domain"
	}
	return s
}

// Target represents an interface to a load balancer target.
type Target interface {
	// Get returns the value for the given key name of  the target's
	// attribute. Keys include:
	//   - alive
	//   - host
	//   - port
	//   - protocol
	//   - type
	Get(key string) string

	// IsAlive returns true if the target is set alive.
	IsAlive() bool

	// IsAvailable tries to dial the target with the given timeout and
	// returns true if the connection succeeded.
	IsAvailable(to time.Duration) bool

	// SetAlive sets the alive attribute of the target.
	SetAlive(v bool)

	// Summary returns a comma-separated string of key-value pairs of the
	// target's attributes.
	Summary() string

	// URL returns a URL formatted string of the target.
	// ("<scheme>://<host>[:<port>]")
	URL() string
}

// target implements the Target interface.
type target struct {
	Port       int
	Protocol   string
	Host       string
	TargetType TargetType
	Alive      bool
	Lock       *sync.RWMutex
}

// NewTarget returns a new Target for the given parameters.
func NewTarget(host string, port int, protocol string) Target {
	targetType := TargetTypeIP
	if net.ParseIP(host) == nil {
		targetType = TargetTypeDomain
	}
	return &target{
		Port:       port,
		Protocol:   protocol,
		Host:       host,
		TargetType: targetType,
		Alive:      true,
		Lock:       new(sync.RWMutex),
	}
}

// NewServiceTarget returns a new service target for the given URL.
func NewServiceTarget(target *url.URL) Target {
	proto := target.Scheme
	port := GetPort(proto)
	host := target.Host
	if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		if i, err := strconv.Atoi(p); err == nil {
			port = i
		}
	}
	return NewTarget(host, port, proto)
}

func (t *target) Get(key string) string {
	v := ""
	switch strings.ToLower(key) {
	case "alive":
		v = fmt.Sprintf("%t", t.Alive)
	case "host":
		v = t.Host
	case "port":
		v = strconv.Itoa(t.Port)
	case "protocol":
		v = t.Protocol
	case "type":
		v = t.TargetType.String()
	}
	return v
}

func (t *target) IsAlive() bool {
	var alive bool
	t.Lock.RLock()
	alive = t.Alive
	t.Lock.RUnlock()
	return alive
}

func (t *target) SetAlive(v bool) {
	t.Lock.Lock()
	t.Alive = v
	t.Lock.Unlock()
}

func (t *target) Summary() string {
	summary := ""
	keys := []string{"alive", "host", "port", "protocol", "type"}
	for i, k := range keys {
		if v := t.Get(k); v != "" {
			summary = fmt.Sprintf("%s%s=%s", summary, k, v)
			if i < (len(keys) - 1) {
				summary = fmt.Sprintf("%s,", summary)
			}
		}
	}
	return summary
}

func (t *target) URL() string {
	url := fmt.Sprintf("%s://%s", t.Protocol, t.Host)
	if t.Port > 0 {
		url = fmt.Sprintf("%s:%d", url, t.Port)
	}
	return url
}

func (t *target) IsAvailable(to time.Duration) bool {
	available := false
	useTls := IsTLS(t.Protocol)
	hostPort := net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
	networks := GetTransport(t.Protocol)
	for _, network := range networks {
		available = dialTarget(network, hostPort, to, useTls)
		if available {
			break
		}
	}
	return available
}

// dialTarget returns true if a successful connection can be made to the address
// using the given network procotol.
func dialTarget(network, addr string, to time.Duration, useTls bool) bool {
	var conn net.Conn
	var err error
	if useTls {
		// We can skip checking the validity of the cert for testing the
		// connection.
		config := tls.Config{InsecureSkipVerify: true}
		conn, err = dialTlsTimeout(network, addr, to, &config)
	} else {
		conn, err = net.DialTimeout(network, addr, to)
	}
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

// dialTlsTimeout is a wrapper for tls.DialWithDialer but with a set timeout.
func dialTlsTimeout(network, addr string, to time.Duration, config *tls.Config) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: to}
	return tls.DialWithDialer(dialer, network, addr, config)
}
