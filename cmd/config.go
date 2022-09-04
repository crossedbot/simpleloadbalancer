package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/crossedbot/simpleloadbalancer/pkg/rules"
)

// LBTarget represents a load balancer target in the configuration. Setting the
//URL will override the other fields.
type LBTarget struct {
	Host string `json:"host"` // Hostname (IP/Domain/etc)
	Port int    `json:"port"` // Port number of the targeted service
	Url  string `json:"url"`  // URL of the targeted service
}

// LBRule represents a load balancer rule in the configuration. Rules are
// commonly used with application load balancer to route strategies to specific
// target groups.
type LBRule struct {
	Action     string              `json:"action"`
	Conditions [][]rules.Condition `json:"conditions"`
}

// LBTargetGroup represents a load balancer target group in the configuration.
// It is a named collection of targets for a given load balancer. Set the Rule
// and protocol fields to route requests for application load balancers.
type LBTargetGroup struct {
	Name     string     `json:"name"`     // TG name
	Protocol string     `json:"protocol"` // TG protocol
	Rule     LBRule     `json:"rule"`     // ALB Rule
	Targets  []LBTarget `json:"targets"`  // The groups targets
}

// Config is the main configuration for this application.
type Config struct {
	Type                string          `json:"type"`     // LB type
	Host                string          `json:"host"`     // Listener host
	Port                int             `json:"port"`     // Listener port
	Protocol            string          `json:"protocol"` // Listener protocol
	TlsEnabled          bool            `json:"tls_enabled"`
	TlsCertFile         string          `json:"tls_cert_file"`
	TlsKeyFile          string          `json:"tls_key_file"`
	Timeout             int64           `json:"timeout"` // Connection timeout
	RequestRate         int64           `json:"request_rate"`
	RequestRateCap      int64           `json:"request_rate_cap"`
	HealthCheckInterval int             `json:"health_check_interval"`
	TargetGroups        []LBTargetGroup `json:"target_groups"`
}

// LoadConfig loads the given JSON file and returns a newly populated Config.
func LoadConfig(fname string) (Config, error) {
	fname = filepath.Clean(fname)
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err := json.Unmarshal(b, &config); err != nil {
		return Config{}, err
	}
	return config, nil
}
