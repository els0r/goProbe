package client

import (
	"io"
	"os"
	"path/filepath"

	"github.com/els0r/goProbe/pkg/api/client"
	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"gopkg.in/yaml.v3"
)

// Client provides a client that calls goProbe's API functions
type Client struct {
	*client.DefaultClient
}

const (
	clientName = "goprobe-client"
)

// NewFromReader creates the client based on configuration read from an io.Reader
func NewFromReader(r io.Reader) (*Client, error) {
	var cfg = new(Config)
	err := yaml.NewDecoder(r).Decode(cfg)
	if err != nil {
		return nil, err
	}
	err = cfg.Validate()
	if err != nil {
		return nil, err
	}
	return NewFromConfig(cfg), nil
}

// New creates a new client instance
func New(addr string, opts ...client.Option) *Client {
	opts = append(opts, client.WithName(clientName))
	return &Client{
		DefaultClient: client.NewDefault(addr, opts...),
	}
}

// NewFromConfig creates the client based on cfg
func NewFromConfig(cfg *Config) *Client {
	if cfg == nil {
		return New(gpapi.DefaultServerAddress)
	}

	c := New(cfg.Addr,
		client.WithRequestLogging(cfg.Log),
		client.WithRequestTimeout(cfg.RequestTimeout),
		client.WithScheme(cfg.Scheme),
		client.WithAPIKey(cfg.Key),
	)

	return c
}

// NewFromConfigFile creates the client based on configuration from a file
func NewFromConfigFile(path string) (*Client, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	return NewFromReader(f)
}
