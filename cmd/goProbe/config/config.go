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
	"os"
	"sync"

	"github.com/els0r/goProbe/pkg/capture"
)

// Mutex exposes a global lock for the configuration
var Mutex sync.Mutex

// Config stores goProbe's configuration
type Config struct {
	DBPath      string                    `json:"db_path"`
	Interfaces  map[string]capture.Config `json:"interfaces"`
	SyslogFlows bool                      `json:"syslog_flows"`
	Logging     LogConfig                 `json:"logging"`
	API         APIConfig                 `json:"api"`
}

// LogConfig stores the logging configuration
type LogConfig struct {
	Destination string `json:"destination"`
	Level       string `json:"level"`
}

// APIConfig stores goProbe's API configuration
type APIConfig struct {
	Port    string `json:"port"`
	Metrics bool   `json:"metrics"`
	Logging bool   `json:"request_logging"`
}

// New creates a new configuration struct with default settings
func New() *Config {
	interfaces := make(map[string]capture.Config)

	return &Config{
		Interfaces: interfaces,
		// default config is syslog
		Logging: LogConfig{
			Destination: "syslog",
			Level:       "info",
		},
		// default API config
		API: APIConfig{
			Port: "6060",
		},
	}
}

// Validate checks all config parameters
func (c *Config) Validate() error {
	if c.DBPath == "" {
		return fmt.Errorf("Database path must not be empty")
	}
	for iface, cc := range c.Interfaces {
		err := cc.Validate()
		if err != nil {
			return fmt.Errorf("Interface '%s' has invalid configuration: %s", iface, err)
		}
	}
	if c.API.Port == "" {
		return fmt.Errorf("No port specified for API server")
	}

	return nil
}

// ParseFile reads in a configuration from a file at `path`.
// If provided, fields are overwritten from the default configuration
func ParseFile(path string) (*Config, error) {
	config := New()

	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	if err := json.NewDecoder(fd).Decode(config); err != nil {
		return nil, err
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	// set the runtime DB path
	if !runtimeDBPath.pathSet {
		runtimeDBPath.path = config.DBPath
	}

	return config, nil
}

var runtimeDBPath = struct {
	pathSet bool
	path    string
}{}

// RuntimeDBPath returns the DB path set at the beginning of program execution
func RuntimeDBPath() string {
	return runtimeDBPath.path
}
