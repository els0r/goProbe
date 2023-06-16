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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"

	"github.com/els0r/goProbe/pkg/defaults"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	json "github.com/json-iterator/go"
	"gopkg.in/yaml.v3"
)

// demoKeys stores the API keys that should, under no circumstance, be used in production.
// They coincide with the keys shown in the README file of goProbe
var demoKeys = map[string]struct{}{
	"da53ae3fb482db63d9606a9324a694bf51f7ad47623c04ab7b97a811f2a78e05": {},
	"9e3b84ae1437a73154ac5c48a37d5085a3f6e68621b56b626f81620de271a2f6": {},
}

// the validator interface is a contract to show if a concrete type is
// configured according to its predefined value range
type validator interface {
	validate() error
}

// Config stores goProbe's configuration
type Config struct {
	sync.Mutex
	DB          DBConfig   `json:"db" yaml:"db"`
	Interfaces  Ifaces     `json:"interfaces" yaml:"interfaces"`
	SyslogFlows bool       `json:"syslog_flows" yaml:"syslog_flows"`
	Logging     LogConfig  `json:"logging" yaml:"logging"`
	API         *APIConfig `json:"api" yaml:"api"`
}

type DBConfig struct {
	Path        string      `json:"path" yaml:"path"`
	EncoderType string      `json:"encoder_type" yaml:"encoder_type"`
	Permissions fs.FileMode `json:"permissions" yaml:"permissions"`
}

type CaptureConfig struct {
	Promisc bool `json:"promisc" yaml:"promisc"`
	// used by the ring buffer in capture
	RingBuffer *RingBufferConfig `json:"ring_buffer" yaml:"ring_buffer"`
}

type RingBufferConfig struct {
	// BlockSize specifies the size of a block, which defines, how many packets
	// can be held within a block
	BlockSize int `json:"block_size" yaml:"block_size"`
	// NumBlocks guides how many blocks are part of the ring buffer
	NumBlocks int `json:"num_blocks" yaml:"num_blocks"`
}

const (
	DefaultBlockSize      int = 1 * 1024 * 1024 // 1 MB
	DefaultRingBufferSize int = 4
)

// Ifaces stores the per-interface configuration
type Ifaces map[string]CaptureConfig

// LogConfig stores the logging configuration
type LogConfig struct {
	Destination string `json:"destination" yaml:"destination"`
	Level       string `json:"level" yaml:"level"`
	Encoding    string `json:"encoding" yaml:"encoding"`
}

// APIConfig stores goProbe's API configuration
type APIConfig struct {
	Addr      string           `json:"addr" yaml:"addr"`
	Metrics   bool             `json:"metrics" yaml:"metrics"`
	Timeout   int              `json:"request_timeout" yaml:"request_timeout"`
	Keys      []string         `json:"keys" yaml:"keys"`
	Discovery *DiscoveryConfig `json:"service_discovery,omitempty" yaml:"service_discovery,omitempty"`
}

// DiscoveryConfig stores access parameters in case goProbe should publish it's API configuration so other services can discover it
type DiscoveryConfig struct {
	Endpoint   string `json:"endpoint" yaml:"endpoint"`
	Identifier string `json:"probe_identifier" yaml:"probe_identifier"`
	Registry   string `json:"registry" yaml:"registry"`
	SkipVerify bool   `json:"skip_verify" yaml:"skip_verify"`
}

// New creates a new configuration struct with default settings
func New() *Config {
	return &Config{
		DB: DBConfig{
			Path:        defaults.DBPath,
			EncoderType: "lz4",
		},
		Interfaces: make(Ifaces),
		Logging: LogConfig{
			Encoding: "logfmt",
			Level:    "info",
		},
	}
}

func (l LogConfig) validate() error {
	return nil
}

func (a APIConfig) validate() error {
	if a.Addr == "" {
		return errors.New("no address specified for API server")
	}
	for _, key := range a.Keys {
		err := checkKeyConstraints(key)
		if err != nil {
			return err
		}
	}
	// check API key constraints
	if a.Timeout < 0 {
		return errors.New("the request timeout must be a positive number > 0")
	}

	// check discovery config
	if a.Discovery != nil {
		return a.Discovery.validate()
	}
	return nil
}

func (d DiscoveryConfig) validate() error {
	if d.Endpoint == "" {
		return errors.New("each probe must publish it's config with a non-empty endpoint on which the API can be reached")
	}
	if d.Identifier == "" {
		return errors.New("each probe must publish it's config with a non-empty identifier if service discvoery is enabled")
	}
	if d.Registry == "" {
		return errors.New("the registry endpoint (configuration store) needs to be specified. Usually this will be a FQDN or an IP:Port pair")
	}
	return nil
}

func (c CaptureConfig) validate() error {
	if c.RingBuffer == nil {
		return errors.New("ring buffer configuration not set")
	}
	return c.RingBuffer.validate()
}

var (
	errorRingBufferBlockSize         = errors.New("ring buffer block size must be a postive number")
	errorRingBufferNumBlocksNegative = errors.New("ring buffer num blocks must be a postive number")
)

func (r *RingBufferConfig) validate() error {
	if r.BlockSize <= 0 {
		return errorRingBufferBlockSize
	}
	if r.NumBlocks <= 0 {
		return errorRingBufferNumBlocksNegative
	}
	return nil
}

// Equals compares c to cfg and returns true if all fields are identical
func (c CaptureConfig) Equals(cfg CaptureConfig) bool {
	return c.Promisc == cfg.Promisc && c.RingBuffer.Equals(cfg.RingBuffer)
}

// Equals compares r to cfg and returns true if all fields are identical
func (r *RingBufferConfig) Equals(cfg *RingBufferConfig) bool {
	if cfg == nil {
		return false
	}
	return r.BlockSize == cfg.BlockSize && r.NumBlocks == cfg.NumBlocks
}

var (
	errorNoInterfacesSpecified = errors.New("no interfaces specified")
)

func (i Ifaces) validate() error {
	if len(i) == 0 {
		return errorNoInterfacesSpecified
	}

	for iface, cc := range i {
		err := cc.validate()
		if err != nil {
			return fmt.Errorf("%s: %w", iface, err)
		}
	}
	return nil
}

// Validate validates the interfaces configuration
func (i Ifaces) Validate() error {
	return i.validate()
}

func (d DBConfig) validate() error {
	if d.Path == "" {
		return errors.New("database path must not be empty")
	}
	_, err := encoders.GetTypeByString(d.EncoderType)
	if err != nil {
		return err
	}
	return nil
}

// Validate checks all config parameters
func (c *Config) Validate() error {
	// run all config subsection validators
	for _, section := range []validator{
		c.DB,
		c.Interfaces,
		c.Logging,
	} {
		err := section.validate()
		if err != nil {
			return err
		}
	}

	// run all config subsection validators for optional sections
	optValidators := []validator{}
	if c.API != nil {
		optValidators = append(optValidators, c.API)
	}
	for _, section := range optValidators {
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

	// we slurp the bytes form the src in order to unmarshal it into JSON or YAML
	// TODO: protect this method from cases where src contains a very large file
	b, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("failed to read bytes: %w", err)
	}

	if jsonErr := json.Unmarshal(b, config); jsonErr != nil {
		yamlErr := yaml.Unmarshal(b, config)
		if yamlErr != nil {
			return nil, fmt.Errorf("failed to unmarshal config: JSON: %v; YAML: %v", jsonErr, yamlErr)
		}
	}

	err = config.Validate()
	if err != nil {
		return nil, err
	}

	return config, nil
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
	return nil
}
