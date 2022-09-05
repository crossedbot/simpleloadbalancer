package loadbalancers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/crossedbot/common/golang/logger"

	"github.com/crossedbot/simpleloadbalancer/pkg/networks"
	"github.com/crossedbot/simpleloadbalancer/pkg/rules"
	"github.com/crossedbot/simpleloadbalancer/pkg/services"
	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
	"github.com/crossedbot/simpleloadbalancer/pkg/templates"
)

var (
	ErrNoTargetsInGroup = errors.New("Target group must contain at least one target")
)

// StopFn is a prototype for a stop routine function.
type StopFn func()

// LoadBalancer represents a common interface for all load balancer types.
type LoadBalancer interface {
	// AddTargetGroup adds the given target group to the load balancer. For
	// network load balancers, there is a single target group. Any
	// additional target groups added to a NLB will simply append the
	// targets to the existing group.
	AddTargetGroup(group *targets.TargetGroup) error

	// HealthCheck starts a routine to passively track the health of the
	// each LB target. It returns a stop function to stop the health check
	// each target's health check routine.
	HealthCheck(interval time.Duration) StopFn

	// GC starts the IP registry garbage collector for each LB target and
	// returns a stop function to stop these routines.
	GC() StopFn

	// Start starts the load balancer on the given listening address and
	// protocol. It returns a stop function to stop listening and exit the
	// routine.
	Start(laddr, protocol string) (StopFn, error)

	// SetErrResponseFormat sets the error response format for the load
	// balancer.
	SetErrResponseFormat(errFmt string)

	// SetTLS enables TLS connections and sets the certificate and private
	// key to the given filenames.
	SetTLS(certFile, keyFile string)

	// Type returns the string representation of the load balancer's type;
	// this is the long name.
	Type() string
}

// appTarget is mapping of an ALB's service pool and other informational fields
// like a name and targeting rules.
type appTarget struct {
	Name        string               // Target name
	Rule        rules.Rule           // Listener rule
	RedirectUrl string               // Redirect URL
	Pool        services.ServicePool // Service pool
}

// appLoadBalancer implements the LoadBalancer interface as application load
// balancer and manages an internal service pool. Application means HTTP
// services.
type appLoadBalancer struct {
	Rate        int64                  // Request Rate
	Capacity    int64                  // Request capacity
	Targets     []appTarget            // Service targets
	TlsEnabled  bool                   // Indicates TLS is enabled
	TlsCertFile string                 // TLS certificate filename
	TlsKeyFile  string                 // TLS private key filename
	ErrRespFmt  targets.ResponseFormat // Error response format
}

// NewApplicationLoadBalancer returns a new Load Balancer for targeted HTTP
// services.
func NewApplicationLoadBalancer(reqRate time.Duration, reqCap int64) LoadBalancer {
	return &appLoadBalancer{
		Rate:       int64(reqRate),
		Capacity:   int64(reqCap),
		ErrRespFmt: targets.DefaultResponseFormat,
	}
}

func (alb *appLoadBalancer) AddTargetGroup(group *targets.TargetGroup) error {
	if len(group.Targets) == 0 {
		return ErrNoTargetsInGroup
	}
	if group.Rule.Action == rules.RuleActionRedirect {
		alb.Targets = append(alb.Targets, appTarget{
			Name:        group.Name,
			Rule:        group.Rule,
			RedirectUrl: group.Targets[0].URL(),
		})
		return nil
	}
	pool := services.New(alb.Rate, alb.Capacity)
	for _, t := range group.Targets {
		t.SetErrResponseFormat(alb.ErrRespFmt)
		if err := pool.AddService(t); err != nil {
			return err
		}
	}
	alb.Targets = append(alb.Targets, appTarget{
		Name: group.Name,
		Rule: group.Rule,
		Pool: pool,
	})
	return nil
}

func (alb *appLoadBalancer) HealthCheck(interval time.Duration) StopFn {
	stops := []StopFn{}
	for _, t := range alb.Targets {
		if t.Pool != nil {
			stops = append(stops,
				StopFn(t.Pool.HealthCheck(interval)))
		}
	}
	return func() {
		for _, fn := range stops {
			fn()
		}
	}
}

func (alb *appLoadBalancer) GC() StopFn {
	stops := []StopFn{}
	for _, t := range alb.Targets {
		if t.Pool != nil {
			stops = append(stops, StopFn(t.Pool.GC()))
		}
	}
	return func() {
		for _, fn := range stops {
			fn()
		}
	}
}

// Redirect sends a redirect to the given URL target with a status code of Moved
// Permanently (HTTP 301). The request's path and query is appended to the URL.
func (alb *appLoadBalancer) Redirect(w http.ResponseWriter, r *http.Request, url string) {
	target := url + r.URL.Path
	if len(r.URL.RawQuery) > 0 {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func (alb *appLoadBalancer) Start(laddr, protocol string) (StopFn, error) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		matchFound := false
		for _, t := range alb.Targets {
			if t.Rule.Matches(r) {
				switch t.Rule.Action {
				case rules.RuleActionForward:
					if t.Pool != nil {
						t.Pool.LoadBalancer()(w, r)
					}
					matchFound = true
				case rules.RuleActionRedirect:
					alb.Redirect(w, r, t.RedirectUrl)
					matchFound = true
				}
				if matchFound {
					break
				}
			}
		}
		if !matchFound {
			handleForbidden(w, alb.ErrRespFmt)
		}
	}
	server := http.Server{
		Addr:    laddr,
		Handler: http.HandlerFunc(handler),
	}
	go func() {
		var err error
		if alb.TlsEnabled {
			err = server.ListenAndServeTLS(alb.TlsCertFile,
				alb.TlsKeyFile)
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			logger.Error(err)
		}
	}()
	return func() { server.Shutdown(context.Background()) }, nil
}

func (alb *appLoadBalancer) SetErrResponseFormat(errFmt string) {
	f := targets.ToResponseFormat(errFmt)
	if f != targets.ResponseFormatUnknown {
		alb.ErrRespFmt = f
	}
}

func (alb *appLoadBalancer) SetTLS(certFile, keyFile string) {
	alb.TlsEnabled = true
	alb.TlsCertFile = certFile
	alb.TlsKeyFile = keyFile
}

func (alb *appLoadBalancer) Type() string {
	return LoadBalancerTypeApp.Long()
}

// handleForbidden handles requests are forbidden from accessing a resource
// (HTTP code 403). In context, this is likely done when an LoadBalancer is
// unable to match any target rules.
func handleForbidden(w http.ResponseWriter, format targets.ResponseFormat) {
	contentType := ""
	msg := ""
	switch format {
	case targets.ResponseFormatHtml:
		contentType = "text/html"
		msg = templates.ForbiddenPage()
	case targets.ResponseFormatJson:
		b, err := json.Marshal(targets.ResponseError{
			Code:    http.StatusForbidden,
			Message: "Forbidden",
		})
		if err == nil {
			contentType = "application/json"
			msg = string(b)
			break
		}
		fallthrough
	default:
		contentType = "text/plain"
		msg = "Forbidden\n"
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, "%s", msg)
}

// netLoadBalancer implements the LoadBalancer interface as a network (E.g. TCP,
// UDP, etc.) load balancer and manages its own network pool.
type netLoadBalancer struct {
	Pool       networks.NetworkPool
	Timeout    time.Duration
	ErrRespFmt targets.ResponseFormat
}

// NewNetworkLoadBalancer returns a LoadBalancer for network-level targets. This
// means services that expect TCP, UDP, whatever connections.
func NewNetworkLoadBalancer(to time.Duration) LoadBalancer {
	return &netLoadBalancer{
		Pool:       networks.New(),
		Timeout:    to,
		ErrRespFmt: targets.DefaultResponseFormat,
	}
}

func (nlb *netLoadBalancer) AddTargetGroup(group *targets.TargetGroup) error {
	for _, t := range group.Targets {
		t.SetErrResponseFormat(nlb.ErrRespFmt)
		if err := nlb.Pool.AddTarget(t, nlb.Timeout); err != nil {
			return err
		}
	}
	return nil
}

func (nlb *netLoadBalancer) HealthCheck(interval time.Duration) StopFn {
	return StopFn(nlb.Pool.HealthCheck(interval))
}

func (nlb *netLoadBalancer) GC() StopFn {
	return StopFn(func() {})
}

func (nlb *netLoadBalancer) Start(laddr, protocol string) (StopFn, error) {
	stopFn, err := nlb.Pool.LoadBalancer(laddr, protocol)
	return StopFn(stopFn), err
}

func (nlb *netLoadBalancer) SetErrResponseFormat(errFmt string) {
	f := targets.ToResponseFormat(errFmt)
	if f != targets.ResponseFormatUnknown {
		nlb.ErrRespFmt = f
	}
}

func (nlb *netLoadBalancer) SetTLS(certFile, keyFile string) {
	// XXX NoOp
}

func (nlb *netLoadBalancer) Type() string {
	return LoadBalancerTypeNet.Long()
}
