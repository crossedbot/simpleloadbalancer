package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/crossedbot/simpleloadbalancer/pkg/rules"
)

// LBTarget represents a load balancer target in the configuration. Setting the
//URL will override the other fields.
type LBTarget struct {
	Host string `json:"host" yaml:"host"` // Hostname (IP/Domain/etc)
	Port int    `json:"port" yaml:"port"` // Port number of the targeted service
	Url  string `json:"url" yaml:"url"`   // URL of the targeted service
}

// LBRule represents a load balancer rule in the configuration. Rules are
// commonly used with application load balancer to route strategies to specific
// target groups.
type LBRule struct {
	Action     string              `json:"action" yaml:"action"`
	Conditions [][]rules.Condition `json:"conditions" yaml:"conditions"`
}

// LBTargetGroup represents a load balancer target group in the configuration.
// It is a named collection of targets for a given load balancer. Set the Rule
// and protocol fields to route requests for application load balancers.
type LBTargetGroup struct {
	Name     string     `json:"name" yaml:"name"`         // TG name
	Protocol string     `json:"protocol" yaml:"protocol"` // TG protocol
	Rule     LBRule     `json:"rule" yaml:"rule"`         // ALB Rule
	Targets  []LBTarget `json:"targets" yaml:"targets"`   // The groups targets
}

// Config is the main configuration for this application.
type Config struct {
	Type                string          `json:"type" yaml:"type"`         // LB type
	Host                string          `json:"host" yaml:"host"`         // Listener host
	Port                int             `json:"port" yaml:"port"`         // Listener port
	Protocol            string          `json:"protocol" yaml:"protocol"` // Listener protocol
	TlsEnabled          bool            `json:"tls_enabled" yaml:"tls_enabled"`
	TlsCertFile         string          `json:"tls_cert_file" yaml:"tls_cert_file"`
	TlsKeyFile          string          `json:"tls_key_file" yaml:"tls_key_file"`
	Timeout             int64           `json:"timeout" yaml:"timeout"` // Connection timeout
	RequestRate         int64           `json:"request_rate" yaml:"request_rate"`
	RequestRateCap      int64           `json:"request_rate_cap" yaml:"request_rate_cap"`
	HealthCheckInterval int             `json:"health_check_interval" yaml:"health_check_interval"`
	TargetGroups        []LBTargetGroup `json:"target_groups" yaml:"target_groups"`
	RespFormat          string          `json:"resp_format" yaml:"resp_format"` // Override LB response format
}

// LoadConfig loads the given JSON file and returns a newly populated Config.
func LoadConfig(fname string) (Config, error) {
	fname = filepath.Clean(fname)
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return Config{}, err
	}
	// Try loading the configuration file as JSON, if this fails fallback to
	// YAML. If that also fails, combine the errors for clarity.
	var config Config
	if jsonErr := json.Unmarshal(b, &config); jsonErr != nil {
		config = Config{}
		if yamlErr := yaml.Unmarshal(b, &config); yamlErr != nil {
			err := fmt.Errorf("JSON: %s; YAML: %s", jsonErr,
				yamlErr)
			return Config{}, err
		}
	}
	return config, nil
}
