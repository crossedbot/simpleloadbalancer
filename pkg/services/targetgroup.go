package services

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

var (
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
		"http":   []string{"tcp"},
		"ssh":    []string{"tcp"},
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
)

func GetPort(protocol string) int {
	return ProtocolPorts[strings.ToLower(protocol)]
}

func GetProtocol(port int) string {
	for proto, curr := range ProtocolPorts {
		if port == curr {
			return proto
		}
	}
	return ""
}

func GetTransport(protocol string) []string {
	return ProtocolTransports[strings.ToLower(protocol)]
}

type TargetType uint32

const (
	TargetTypeIP TargetType = iota + 1
	TargetTypeDomain
)

type Target struct {
	Name       string
	Port       int
	Protocol   string
	Target     string
	TargetType TargetType
}

func NewTarget(name string, target *url.URL) *Target {
	proto := target.Scheme
	port := GetPort(proto)
	host := target.Host
	if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		if i, err := strconv.Atoi(p); err == nil {
			port = i
		}
	}
	targetType := TargetTypeIP
	if net.ParseIP(host) == nil {
		targetType = TargetTypeDomain
	}
	return &Target{
		Name:       name,
		Port:       port,
		Protocol:   proto,
		Target:     host,
		TargetType: targetType,
	}
}

func (t *Target) URL() (*url.URL, error) {
	if t.Protocol == "" {
		return nil, ErrMissingProtocol
	}
	s := t.String()
	return url.Parse(s)
}

func (t *Target) String() string {
	s := t.Target
	proto := t.Protocol
	if t.Port > 0 {
		s = fmt.Sprintf("%s:%d", s, t.Port)
		if proto == "" {
			proto = GetProtocol(t.Port)
		}
	}
	if proto != "" {
		s = fmt.Sprintf("%s://%s", proto, s)
	}
	return s
}
