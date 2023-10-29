package client

import (
	"context"

	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/api/client"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/fako1024/httpc"
)

// Client denotes a global query client
type Client struct {
	*client.DefaultClient
}

const (
	clientName = "global-query-client"
)

// New creates a new client for the global-query API
func New(addr string, opts ...client.Option) *Client {
	opts = append(opts, client.WithName(clientName))
	return &Client{
		DefaultClient: client.NewDefault(addr, opts...),
	}
}

// Run implements the query.Runner interface
func (c *Client) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	return c.Query(ctx, args)
}

// Query performs the global query and returns its result
func (c *Client) Query(ctx context.Context, args *query.Args) (*results.Result, error) {
	// use a copy of the arguments, since some fields are modified by the client
	queryArgs := *args

	// whatever happens, the results are expected to be returned in json
	queryArgs.Format = "json"

	if queryArgs.Caller == "" {
		queryArgs.Caller = clientName
	}

	var res = new(results.Result)

	req := c.Modify(ctx,
		httpc.NewWithClient("POST", c.NewURL(api.QueryRoute), c.Client()).
			EncodeJSON(queryArgs).
			ParseJSON(res),
	)

	err := req.RunWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return res, nil
}
