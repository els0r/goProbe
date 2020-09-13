/////////////////////////////////////////////////////////////////////////////////
//
// config.go
//
// Written by Lorenz Breidenbach lob@open.ch, December 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// Package config is for parsing goprobe config files.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
)

// demoKeys stores the API keys that should, under no circumstance, be used in production.
// They coincide with the keys shown in the README file of goProbe
var demoKeys = map[string]struct{}{
	"da53ae3fb482db63d9606a9324a694bf51f7ad47623c04ab7b97a811f2a78e05": struct{}{},
	"9e3b84ae1437a73154ac5c48a37d5085a3f6e68621b56b626f81620de271a2f6": struct{}{},
}

// the validator interface is a contract to show if a concrete type is
// configured according to its predefined value range
type validator interface {
	validate() error
}

// Config stores goProbe's configuration
type Config struct {
	sync.Mutex
	DBPath      string    `json:"db_path"`
	Interfaces  Ifaces    `json:"interfaces"`
	SyslogFlows bool      `json:"syslog_flows"`
	Logging     LogConfig `json:"logging"`
	API         APIConfig `json:"api"`
	EncoderType string    `json:"encoder_type"`
}

// Ifaces stores the per-interface configuration
type Ifaces map[string]capture.Config

// LogConfig stores the logging configuration
type LogConfig struct {
	Destination string `json:"destination"`
	Level       string `json:"level"`
}

// APIConfig stores goProbe's API configuration
type APIConfig struct {
	Host      string           `json:"host"`
	Port      string           `json:"port"`
	Metrics   bool             `json:"metrics"`
	Logging   bool             `json:"request_logging"`
	Timeout   int              `json:"request_timeout"`
	Keys      []string         `json:"keys"`
	Discovery *DiscoveryConfig `json:"service_discovery,omitempty"`
}

// DiscoveryConfig stores access parameters in case goProbe should publish it's API configuration so other services can discover it
type DiscoveryConfig struct {
	Endpoint   string `json:"endpoint"`
	Identifier string `json:"probe_identifier"`
	Registry   string `json:"registry"`
	SkipVerify bool   `json:"skip_verify"`
}

// New creates a new configuration struct with default settings
func New() *Config {
	return &Config{
		Interfaces: make(Ifaces),
		// default config is syslog
		Logging: LogConfig{
			Destination: "syslog",
			Level:       "info",
		},
		// default API config
		API: APIConfig{
			Host: "localhost",
			Port: "6060",
		},
		EncoderType: "lz4",
	}
}

func (l LogConfig) validate() error {
	return nil
}

func (a APIConfig) validate() error {
	if a.Port == "" {
		return fmt.Errorf("No port specified for API server")
	}
	for _, key := range a.Keys {
		err := checkKeyConstraints(key)
		if err != nil {
			return err
		}
	}
	// check API key constraints
	if a.Timeout < 0 {
		return fmt.Errorf("The request timeout must be a positive number > 0")
	}

	// check discovery config
	if a.Discovery != nil {
		return a.Discovery.validate()
	}
	return nil
}

func (d DiscoveryConfig) validate() error {
	if d.Endpoint == "" {
		return fmt.Errorf("Each probe must publish it's config with a non-empty endpoint on which the API can be reached")
	}
	if d.Identifier == "" {
		return fmt.Errorf("Each probe must publish it's config with a non-empty identifier if service discvoery is enabled")
	}
	if d.Registry == "" {
		return fmt.Errorf("The registry endpoint (configuration store) needs to be specified. Usually this will be a FQDN or an IP:Port pair")
	}
	return nil
}

func (i Ifaces) validate() error {
	if len(i) == 0 {
		return fmt.Errorf("No interfaces were specified")
	}

	for iface, cc := range i {
		err := cc.Validate()
		if err != nil {
			return fmt.Errorf("Interface '%s' has invalid configuration: %s", iface, err)
		}
	}
	return nil
}

func (c *Config) validate() error {
	if c.DBPath == "" {
		return fmt.Errorf("Database path must not be empty")
	}
	_, err := encoders.GetTypeByString(c.EncoderType)
	if err != nil {
		return err
	}
	return nil
}

// Validate checks all config parameters
func (c *Config) Validate() error {
	// run all config subsection validators
	for _, section := range []validator{
		c,
		c.Interfaces,
		c.Logging,
		c.API,
	} {
		err := section.validate()
		if err != nil {
			return err
		}
	}
	return nil
}

// ParseFile reads in a configuration from a file at `path`.
// If provided, fields are overwritten from the default configuration
func ParseFile(path string) (*Config, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	return Parse(fd)
}

// Parse attempts to read the configuration from an io.Reader
func Parse(src io.Reader) (*Config, error) {
	config := New()
	if err := json.NewDecoder(src).Decode(config); err != nil {
		return nil, err
	}

	err := config.Validate()
	if err != nil {
		return nil, err
	}

	// set the runtime DB path
	if !runtimeDBPath.set {
		runtimeDBPath.path = config.DBPath
		runtimeDBPath.set = true
	}

	return config, nil
}

var runtimeDBPath = struct {
	set  bool
	path string
}{}

// RuntimeDBPath returns the DB path set at the beginning of program execution
func RuntimeDBPath() string {
	return runtimeDBPath.path
}

func checkKeyConstraints(key string) error {
	// enforce long API keys (e.g. SHA256)
	if len(key) < 32 {
		return fmt.Errorf("API key '%s' considered insecure: insufficient key length %d", key, len(key))
	}

	// check if someone actually used one of the demo keys
	_, usedIt := demoKeys[key]
	if usedIt {
		return fmt.Errorf("API key '%s' considered compromised: identical to demo-key in README.md", key)
	}

	// TODO: consider to check entropy of key
	return nil
}
