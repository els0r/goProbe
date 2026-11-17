package client

import (
	"errors"
	"time"
)

// Config specifies the configurable parts of the client
type Config struct {
	Scheme string `json:"scheme" yaml:"scheme"`
	Addr   string `json:"addr" yaml:"addr"`
	Key    string `json:"key,omitempty" yaml:"key,omitempty"`
	// Token is presented as a Bearer credential. It takes precedence over Key
	Token string `json:"token,omitempty" yaml:"token,omitempty"`

	RequestTimeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`

	Log bool `json:"log" yaml:"log"`
}

var (
	ErrorEmptyAddress = errors.New("no endpoint address (host:port) provided") // ErrorEmptyAddress : Denotes that an empty config has been provided
)

// Validate validates the configuration
func (cfg *Config) Validate() error {
	if cfg.Addr == "" {
		return ErrorEmptyAddress
	}
	return nil
}
