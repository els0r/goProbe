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
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"golang.org/x/net/bpf"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"

	"github.com/els0r/goProbe/v4/pkg/defaults"
	"github.com/els0r/goProbe/v4/pkg/goDB/encoder/encoders"
)

const (
	// ServiceName is the name of the service as it will show up in telemetry such as metrics, logs, traces, etc.
	ServiceName = "goprobe"

	// InterfaceAuto denotes the special (fake) interface name to auto-detect interfaces
	InterfaceAuto = "autodetect"

	maxConfigSize = 16 * 1024 * 1024 // 16 MiB
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
	DB            DBConfig            `json:"db" yaml:"db" mapstructure:"db"`
	AutoDetection AutoDetectionConfig `json:"autodetection" yaml:"autodetection" mapstructure:"autodetection" doc:"Configures auto-detection of interfaces"`
	Interfaces    Ifaces              `json:"interfaces" yaml:"interfaces" mapstructure:"interfaces"`
	SyslogFlows   bool                `json:"syslog_flows" yaml:"syslog_flows" mapstructure:"syslog_flows"`
	API           APIConfig           `json:"api" yaml:"api" mapstructure:"api"`
	LocalBuffers  LocalBufferConfig   `json:"local_buffers" yaml:"local_buffers" mapstructure:"local_buffers"`
}

// DBConfig stores the local on-disk database configuration
type DBConfig struct {
	Path        string      `json:"path" yaml:"path" mapstructure:"path"`
	EncoderType string      `json:"encoder_type" yaml:"encoder_type" mapstructure:"encoder_type"`
	Permissions fs.FileMode `json:"permissions" yaml:"permissions" mapstructure:"permissions"`
}

// CaptureConfig stores the capture / buffer related configuration for an individual interface
type CaptureConfig struct {
	// IgnoreVLANs: enables / disables skipping of VLAN-tagged packets
	IgnoreVLANs bool `json:"ignore_vlans" yaml:"ignore_vlans" mapstructure:"ignore_vlans" doc:"Enables / disables skipping of VLAN-tagged packets on interface" example:"true"`
	// Promisc: enables / disables promiscuous capture mode
	Promisc bool `json:"promisc" yaml:"promisc" mapstructure:"promisc" doc:"Enables / disables promiscuous capture mode on interface" example:"true"`
	// RingBuffer: denotes the kernel ring buffer configuration of this interface
	RingBuffer *RingBufferConfig `json:"ring_buffer" yaml:"ring_buffer" mapstructure:"ring_buffer" doc:"Kernel ring buffer configuration for interface"`
	// ExtraBPFFilters: allows setting additional BPF filter instructions during capture
	ExtraBPFFilters []bpf.RawInstruction `json:"extra_bpf_filters" yaml:"extra_bpf_filters" mapstructure:"extra_bpf_filters" doc:"Extra BPF filter instructions to be applied during capture"`
	// Disable: explicitly disables capture on this interface (e.g. in conjunction with the `autodetect` option)
	Disable bool `json:"disable" yaml:"disable" mapstructure:"disable" doc:"Explicitly disables capture on this interface" example:"true"`
}

// LocalBufferConfig stores the shared local in-memory buffer configuration
type LocalBufferConfig struct {
	// SizeLimit denotes the maximum size of the local buffers (globally)
	// used to continue capturing while the capture is (b)locked
	SizeLimit int `json:"size_limit" yaml:"size_limit" mapstructure:"size_limit"`

	// NumBuffers denotes the number of buffers (and hence maximum concurrency
	// of Status() calls). This should be left at default unless absolutely required
	NumBuffers int `json:"num_buffers" yaml:"num_buffers" mapstructure:"num_buffers"`
}

// RingBufferConfig stores the kernel ring buffer related configuration for an individual interface
type RingBufferConfig struct {
	// BlockSize: specifies the size of a block, which defines how many packets can be held within a block
	BlockSize int `json:"block_size" yaml:"block_size" mapstructure:"block_size" doc:"Size of a block, which defines how many packets can be held within a block" example:"1048576" minimum:"1"`

	// NumBlocks: guides how many blocks are part of the ring buffer
	NumBlocks int `json:"num_blocks" yaml:"num_blocks" mapstructure:"num_blocks" doc:"Guides how many blocks are part of the ring buffer" example:"4" minimum:"1"`
}

const (
	DefaultRingBufferBlockSize   int = 1 * 1024 * 1024  // DefaultRingBufferBlockSize : 1 MB
	DefaultRingBufferNumBlocks   int = 4                // DefaultRingBufferNumBlocks : 4
	DefaultLocalBufferSizeLimit  int = 64 * 1024 * 1024 // DefaultLocalBufferSizeLimit : 64 MB (globally, not per interface)
	DefaultLocalBufferNumBuffers int = 1                // DefaultLocalBufferNumBuffers : 1 (should suffice)
)

// IfaceName denotes the name of a network interface
// Can also be a regexp matcher (e.g. "/^eth[0-9]+$/")
type IfaceName = string

// Ifaces maps interface names to their capture configuration
type Ifaces map[IfaceName]CaptureConfig

// InterfaceConfigs stores the configuration for all interfaces
type InterfaceConfigs struct {
}

// AutoDetectionConfig stores the auto-detection configuration for interfaces
type AutoDetectionConfig struct {
	// Enabled: enables / disables auto-detection of interfaces
	Enabled bool `json:"enabled" yaml:"enabled" mapstructure:"enabled" doc:"Enables / disables auto-detection of interfaces" example:"true"`
	// Exclude: a list of interface names to exclude from auto-detection
	Exclude []IfaceName `json:"exclude" yaml:"exclude" mapstructure:"exclude" doc:"A list of interface names to exclude from auto-detection" example:"eth0,lo"`
}

// LogConfig stores the logging configuration
type LogConfig struct {
	Destination string `json:"destination" yaml:"destination" mapstructure:"destination"`
	Level       string `json:"level" yaml:"level" mapstructure:"level"`
	Encoding    string `json:"encoding" yaml:"encoding" mapstructure:"encoding"`
}

// QueryRateLimitConfig contains query rate limiting related config arguments / parameters
type QueryRateLimitConfig struct {
	MaxReqPerSecond rate.Limit `json:"max_req_per_sec" yaml:"max_req_per_sec" mapstructure:"max_req_per_sec"`
	MaxBurst        int        `json:"max_burst" yaml:"max_burst" mapstructure:"max_burst"`
	MaxConcurrent   int        `json:"max_concurrent" yaml:"max_concurrent" mapstructure:"max_concurrent"`
}

// APIConfig stores goProbe's API configuration
type APIConfig struct {
	Addr                string               `json:"addr" yaml:"addr" mapstructure:"addr"`
	Metrics             bool                 `json:"metrics" yaml:"metrics" mapstructure:"metrics"`
	DisableIfaceMetrics bool                 `json:"disable_iface_metrics" yaml:"disable_iface_metrics" mapstructure:"disable_iface_metrics"`
	Profiling           bool                 `json:"profiling" yaml:"profiling" mapstructure:"profiling"`
	Timeout             int                  `json:"request_timeout" yaml:"request_timeout" mapstructure:"request_timeout"`
	Keys                []string             `json:"keys" yaml:"keys" mapstructure:"keys"`
	QueryRateLimit      QueryRateLimitConfig `json:"query_rate_limit" yaml:"query_rate_limit" mapstructure:"query_rate_limit"`
}

// newDefault creates a new configuration struct with default settings
func newDefault() *Config {
	return &Config{
		DB: DBConfig{
			Path:        defaults.DBPath,
			EncoderType: "lz4",
		},
		Interfaces: make(Ifaces),
	}
}

func (l LogConfig) validate() error {
	return nil
}

var (
	errorInvalidAPITimeout        = errors.New("the request timeout must be a positive number")
	errorInvalidAPIQueryRateLimit = errors.New("the query rate limit values must both be positive numbers")
)

func (a APIConfig) validate() error {
	if a.Addr == "" {
		return nil
	}

	if (a.QueryRateLimit.MaxReqPerSecond <= 0. && a.QueryRateLimit.MaxBurst > 0) ||
		(a.QueryRateLimit.MaxReqPerSecond > 0. && a.QueryRateLimit.MaxBurst <= 0) {
		return errorInvalidAPIQueryRateLimit
	}
	if a.QueryRateLimit.MaxConcurrent < 0 {
		return errorInvalidAPIQueryRateLimit
	}
	for _, key := range a.Keys {
		err := checkKeyConstraints(key)
		if err != nil {
			return err
		}
	}
	// check API key constraints
	if a.Timeout < 0 {
		return errorInvalidAPITimeout
	}
	return nil
}

var (
	errorLocalBufferSize       = errors.New("local buffer size must be a positive number")
	errorLocalBufferNumBuffers = errors.New("number of local buffers must be a positive number")
)

func (l LocalBufferConfig) validate() error {
	if l.NumBuffers == 0 && l.SizeLimit == 0 {
		return nil
	}
	if l.SizeLimit < 0 {
		return errorLocalBufferSize
	}
	if l.NumBuffers < 0 {
		return errorLocalBufferNumBuffers
	}
	return nil
}

var (
	errorNoRingBufferConfig          = errors.New("no ring buffer configuration specified")
	errorSettingsWithCaptureDisabled = errors.New("capture settings specified, but capture is disabled")
)

func (c CaptureConfig) validate() error {

	// Ensure no other settings are set when capture is disabled
	if c.Disable {
		if c.RingBuffer != nil ||
			len(c.ExtraBPFFilters) > 0 ||
			c.Promisc || c.IgnoreVLANs {
			return errorSettingsWithCaptureDisabled
		}
		return nil
	}

	if c.RingBuffer == nil {
		return errorNoRingBufferConfig
	}
	return c.RingBuffer.validate()
}

var (
	errorRingBufferBlockSize = errors.New("ring buffer block size must be a postive number")
	errorRingBufferNumBlocks = errors.New("ring buffer num blocks must be a postive number")
)

func (r *RingBufferConfig) validate() error {
	if r.BlockSize <= 0 {
		return errorRingBufferBlockSize
	}
	if r.NumBlocks <= 0 {
		return errorRingBufferNumBlocks
	}
	return nil
}

// Equals compares c to cfg and returns true if all fields are identical
func (c CaptureConfig) Equals(cfg CaptureConfig) bool {
	return c.Promisc == cfg.Promisc &&
		c.RingBuffer.Equals(cfg.RingBuffer)
}

// Equals compares r to cfg and returns true if all fields are identical
func (r *RingBufferConfig) Equals(cfg *RingBufferConfig) bool {
	if cfg == nil {
		return false
	}
	return r.BlockSize == cfg.BlockSize && r.NumBlocks == cfg.NumBlocks
}

var (
	errorNoInterfacesSpecified       = errors.New("no interfaces specified")
	errorIfaceMustBeDisabledWithAuto = errors.New("interface must be disabled when autodetect interface is configured")
)

func (i Ifaces) validate() error {
	if len(i) == 0 {
		return errorNoInterfacesSpecified
	}

	// Ensure that other interfaces are disabled (veto-only) when auto-detect is used
	_, hasAuto := i.HasAutoDetect()
	for iface, cc := range i {
		err := cc.validate()
		if err != nil {
			return fmt.Errorf("%s: %w", iface, err)
		}

		if hasAuto && iface != InterfaceAuto && !cc.Disable {
			return fmt.Errorf("%s: %w", iface, errorIfaceMustBeDisabledWithAuto)
		}
	}
	return nil
}

// Validate validates the interfaces configuration
func (i Ifaces) Validate() error {
	return i.validate()
}

// HasAutoDetect returns true if c matches the auto-detect default configuration
func (i Ifaces) HasAutoDetect() (CaptureConfig, bool) {
	config, exists := i[InterfaceAuto]
	return config, exists
}

// Matcher creates an IfaceMatcher from the Ifaces configuration
func (i Ifaces) Matcher() (*IfaceMatcher, bool, error) {
	matcher := IfaceMatcher{
		Ifaces:         i,
		regexpMatchers: make(map[*regexp.Regexp]CaptureConfig),
	}

	for k := range i {
		if IsRegexpInterfaceMatcher(k) {
			re, err := regexp.Compile(k[1 : len(k)-1])
			if err != nil {
				return nil, false, fmt.Errorf("invalid regexp interface matcher '%s': %w", k, err)
			}

			matcher.regexpMatchers[re] = i[k]
		}
	}

	return &matcher, len(matcher.regexpMatchers) > 0, nil
}

// IfaceMatcher stores the per-interface configuration (including regexp matchers)
type IfaceMatcher struct {
	Ifaces

	regexpMatchers map[*regexp.Regexp]CaptureConfig
}

// FindCaptureConfig attempts to find a matching interface configuration for the given interface name
func (m *IfaceMatcher) FindCaptureConfig(iface string) (CaptureConfig, bool) {

	// first check for direct match
	if cfg, exists := m.Ifaces[iface]; exists {
		return cfg, true
	}

	// then check regexp matchers
	for re, cfg := range m.regexpMatchers {
		if re.MatchString(iface) {
			return cfg, true
		}
	}

	return CaptureConfig{}, false
}

var (
	errorEmptyDBPath = errors.New("database path must not be empty")
)

func (d DBConfig) validate() error {
	if d.Path == "" {
		return errorEmptyDBPath
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
		c.API,
		c.LocalBuffers,
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
	fd, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := fd.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	return Parse(fd)
}

var (
	errorUnmarshalConfig = errors.New("failed to unmarshal config")
)

// Parse attempts to read the configuration from an io.Reader
func Parse(src io.Reader) (*Config, error) {
	config := newDefault()

	// Slurp the bytes form the src in order to unmarshal it into JSON or YAML
	// In order to protect this method from cases where src contains a very large file we limit reading
	// to a maximum size of <maxConfigSize>
	limitedReader := &io.LimitedReader{R: src, N: maxConfigSize}
	b, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read bytes: %w", err)
	}

	if jsonErr := jsoniter.Unmarshal(b, config); jsonErr != nil {
		yamlErr := yaml.Unmarshal(b, config)
		if yamlErr != nil {
			return nil, fmt.Errorf("%w: JSON: %w; YAML: %w", errorUnmarshalConfig, jsonErr, yamlErr)
		}
	}

	err = config.Validate()
	if err != nil {
		return nil, err
	}

	return config, nil
}

// IsRegexpInterfaceMatcher determines if the given interface name is a regexp matcher
func IsRegexpInterfaceMatcher(iface IfaceName) bool {
	return strings.HasPrefix(iface, "/") && strings.HasSuffix(iface, "/")
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
