package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/fako1024/httpc"
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

const defaultRequestTimeout = 30 * time.Second

// New creates a new goProbe API client
func New() *Client {
	return &Client{
		client:   http.DefaultClient,
		scheme:   "http",
		hostAddr: "localhost:6061",
		timeout:  defaultRequestTimeout,
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
	if timeout > 0 {
		c.timeout = timeout
	}
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

func (c *Client) authorize(ctx context.Context, req *httpc.Request) *httpc.Request {
	// TODO: this should go into the transport as well
	if c.logRequests {
		logger := logging.WithContext(ctx).With("method", req.GetMethod(), "url", req.GetURI())
		logger.Info("creating new request")
	}
	if c.key != "" {
		return req.Headers(map[string]string{
			"Authorization": fmt.Sprintf("digest %s", c.key),
		})
	}
	return req
}

func (c *Client) newURL(path string) string {
	return fmt.Sprintf("%s://%s/%s", c.scheme, c.hostAddr, path)
}

// Run implements the query.Runner interface
func (c *Client) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	// use a copy of the arguments, since some fields are modified by the client
	argsQuery := *args
	return c.Query(ctx, &argsQuery)
}

// Query runs a query on the API endpoint
func (c *Client) Query(ctx context.Context, args *query.Args) (*results.Result, error) {
	// whatever happens, the results are expected to be returned in json
	args.Format = "json"

	// we need more results before truncating
	if args.NumResults < query.DefaultNumResults {
		args.NumResults = query.DefaultNumResults
	}

	var res = new(results.Result)

	req := c.authorize(ctx, httpc.NewWithClient("POST", c.newURL(queryPath), c.client).
		EncodeJSON(args).
		Timeout(c.timeout).
		RetryBackOffErrFn(func(resp *http.Response, _ error) bool {
			return resp.StatusCode == http.StatusTooManyRequests
		}).
		ParseJSON(res),
	)
	err := req.RunWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run request: %w", err)
	}
	return res, nil
}
