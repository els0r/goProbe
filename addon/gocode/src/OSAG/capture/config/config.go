/////////////////////////////////////////////////////////////////////////////////
//
// config.go
//
// Written by Lorenz Breidenbach lob@open.ch, December 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// Package for parsing goprobe config files.
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"OSAG/goProbe"
)

type Config struct {
	DBPath      string                           `json:"db_path"`
	Interfaces  map[string]goProbe.CaptureConfig `json:"interfaces"`
	SyslogFlows bool                             `json:"syslog_flows"`
}

func NewConfig() *Config {
	interfaces := make(map[string]goProbe.CaptureConfig)
	return &Config{
		Interfaces: interfaces,
	}
}

func (c Config) Validate() error {
	if c.DBPath == "" {
		return fmt.Errorf("Database path must not be empty")
	}
	for iface, cc := range c.Interfaces {
		err := cc.Validate()
		if err != nil {
			return fmt.Errorf("Interface '%s' has invalid configuration: %s", iface, err)
		}
	}
	return nil
}

func ParseFile(path string) (*Config, error) {
	config := NewConfig()

	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	data, err := ioutil.ReadAll(fd)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}
