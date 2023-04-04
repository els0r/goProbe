package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	jsoniter "github.com/json-iterator/go"
	"gopkg.in/yaml.v3"
)

// Client provides a client that calls goProbe's API functions
type Client struct {
	client  *http.Client
	timeout time.Duration

	scheme   string
	hostAddr string
	key      string

	logRequests bool
}

const (
	apiPath = "api/v1"

	queryPath = apiPath + "/_query"
)

// New creates a new goProbe API client
func New() *Client {
	return &Client{
		client:   http.DefaultClient,
		scheme:   "http",
		hostAddr: "localhost:6061",
		timeout:  30 * time.Second,
	}
}

// At sets the API address to addr
func (c *Client) At(addr string) *Client {
	if addr != "" {
		c.hostAddr = addr
	}
	return c
}

// APIKey sets the API key, so that it's presented as part
// of the 'Authorization' header
func (c *Client) APIKey(key string) *Client {
	if key != "" {
		c.key = key
	}
	return c
}

// LogRequests toggles request logging
func (c *Client) LogRequests(b bool) *Client {
	c.logRequests = b
	return c
}

// RequestTimeout sets the request timeout
func (c *Client) RequestTimeout(timeout time.Duration) *Client {
	c.timeout = timeout
	return c
}

// Scheme sets the http scheme to be used
func (c *Client) Scheme(scheme string) *Client {
	if scheme != "" {
		c.scheme = scheme
	}
	return c
}

// FromReader reads the client configuration from an io.Reader
func (c *Client) FromReader(r io.Reader) (*Client, error) {
	var cfg = new(Config)
	err := yaml.NewDecoder(r).Decode(cfg)
	if err != nil {
		return c, err
	}
	err = cfg.Validate()
	if err != nil {
		return c, err
	}
	return c.FromConfig(cfg), nil
}

// FromConfig assigns the client Config to the client
func (c *Client) FromConfig(cfg *Config) *Client {
	if cfg == nil {
		return c
	}
	return c.At(cfg.Addr).
		Scheme(cfg.Scheme).
		APIKey(cfg.Key).
		LogRequests(cfg.Log).
		RequestTimeout(cfg.RequestTimeout)
}

// FromConfigFile reads the client configuration from a file
func (c *Client) FromConfigFile(path string) (*Client, error) {
	f, err := os.Open(path)
	if err != nil {
		return c, err
	}
	defer f.Close()

	return c.FromReader(f)
}

func (c *Client) newAuthorizedRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := fmt.Sprintf("%s://%s/%s", c.scheme, c.hostAddr, path)

	// TODO: this should go into the transport as well
	if c.logRequests {
		logger := logging.WithContext(ctx).With("method", method, "url", url)
		logger.Info("creating new request")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if c.key != "" {
		req.Header.Add("Authorization", fmt.Sprintf("digest %s", c.key))
	}

	return req, nil
}

// Run implements the query.Runner interface
func (c *Client) Run(ctx context.Context, stmt *query.Statement) ([]results.Result, error) {
	return c.Query(ctx, stmt)
}

// Query runs a query on the API endpoint
func (c *Client) Query(ctx context.Context, stmt *query.Statement) ([]results.Result, error) {
	var buf = new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(stmt)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query statement: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := c.newAuthorizedRequest(ctx, "POST", queryPath, buf)
	if err != nil {
		return nil, err
	}

	var res []results.Result
	for {
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			err = jsoniter.NewDecoder(resp.Body).Decode(res)
			if err != nil {
				return nil, fmt.Errorf("failed to decode results: %w", err)
			}
			return res, nil
		case http.StatusTooManyRequests:
			// TODO: should be backoff and live within a custom transport implementation
			time.Sleep(10 * time.Second)
			continue
		default:
			return nil, fmt.Errorf("%s", resp.Status)
		}
	}
}
