package targets

import (
	"net/url"

	"github.com/crossedbot/simpleloadbalancer/pkg/rules"
)

// TargetGroup represents a group of targets.
type TargetGroup struct {
	Name       string         // Group name
	Protocol   string         // Common group protocol
	Rule       rules.Rule     // Request rule
	ErrRespFmt ResponseFormat // Set targets response format
	Targets    []Target       // List of targets
}

// NewTargetGroup returns a new TargetGroup.
func NewTargetGroup(name, protocol string, rule rules.Rule, target ...Target) *TargetGroup {
	return &TargetGroup{
		Name:       name,
		Protocol:   protocol,
		Rule:       rule,
		ErrRespFmt: DefaultResponseFormat,
		Targets:    append([]Target{}, target...),
	}
}

// AddServiceTarget adds a new target as a service via a given URL.
func (tg *TargetGroup) AddServiceTarget(target *url.URL) {
	t := NewServiceTarget(target)
	t.SetErrResponseFormat(tg.ErrRespFmt)
	tg.Targets = append(tg.Targets, t)
}

// AddTarget adds a new target via a given host and port.
func (tg *TargetGroup) AddTarget(host string, port int) {
	t := NewTarget(host, port, tg.Protocol)
	t.SetErrResponseFormat(tg.ErrRespFmt)
	tg.Targets = append(tg.Targets, t)
}

// SetErrResponseFormat sets the error response format for the target group. If
// the format is unknown, the formatting will remain unchanged.
func (tg *TargetGroup) SetErrResponseFormat(errFmt ResponseFormat) {
	if errFmt.String() != ResponseFormatUnknown.String() {
		tg.ErrRespFmt = errFmt
	}
}
