package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/els0r/goProbe/pkg/global-query/api"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/fako1024/httpc"
)

type Client struct {
	scheme   string
	hostAddr string

	client *http.Client

	timeout        time.Duration
	requestLogging bool
}

// NewClient creates a new client for the global-query API
func NewClient(addr string) *Client {
	return &Client{
		client:   http.DefaultClient,
		scheme:   "http",
		hostAddr: addr,
	}
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

func (c *Client) newURL(path string) string {
	return fmt.Sprintf("%s://%s%s", c.scheme, c.hostAddr, path)
}

func (c *Client) modify(ctx context.Context, req *httpc.Request) *httpc.Request {
	if c.requestLogging {
		req = req.ModifyRequest(func(req *http.Request) error {
			logging.WithContext(ctx).WithGroup("req").With("method", req.Method, "url", req.URL).Infof("sending request")
			return nil
		})
	}
	if c.timeout > 0 {
		req = req.Timeout(c.timeout)
	}
	return req
}

// Run implements the query.Runner interface
func (c *Client) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	return c.Query(ctx, args)
}

func (c *Client) Query(ctx context.Context, args *query.Args) (*results.Result, error) {
	// use a copy of the arguments, since some fields are modified by the client
	queryArgs := *args
	// whatever happens, the results are expected to be returned in json
	queryArgs.Format = "json"

	var res = new(results.Result)

	req := httpc.NewWithClient("POST", c.newURL(api.QueryRoute), c.client).
		EncodeJSON(queryArgs).
		RetryBackOffErrFn(func(resp *http.Response, _ error) bool {
			return resp.StatusCode == http.StatusTooManyRequests
		}).
		ParseJSON(res)

	// attach timeout and logging if configured
	req = c.modify(ctx, req)

	err := req.RunWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run request: %w", err)
	}
	return res, nil
}
