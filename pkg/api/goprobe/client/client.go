package client

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/els0r/goProbe/pkg/api/client"
	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/fako1024/httpc"
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
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return NewFromReader(f)
}

// Run implements the query.Runner interface
func (c *Client) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	return c.Query(ctx, args)
}

// Query runs a query on the API endpoint
func (c *Client) Query(ctx context.Context, args *query.Args) (*results.Result, error) {
	// use a copy of the arguments, since some fields are modified by the client
	queryArgs := *args
	// whatever happens, the results are expected to be returned in json
	queryArgs.Format = "json"

	// we need more results before truncating
	if args.NumResults < query.DefaultNumResults {
		args.NumResults = query.DefaultNumResults
	}

	var res = new(results.Result)

	req := c.Modify(ctx,
		httpc.NewWithClient("POST", c.NewURL(gpapi.QueryRoute), c.Client()).
			EncodeJSON(queryArgs).
			ParseJSON(res),
	)
	err := req.RunWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run request: %w", err)
	}

	return res, nil
}
