package client

import (
	"context"
	"fmt"

	"github.com/els0r/goProbe/pkg/api/client"
	gqapi "github.com/els0r/goProbe/pkg/api/globalquery"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/fako1024/httpc"
)

type Client struct {
	*client.DefaultClient
}

const (
	clientName = "global-query-client"
)

// NewClient creates a new client for the global-query API
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

func (c *Client) Query(ctx context.Context, args *query.Args) (*results.Result, error) {
	// use a copy of the arguments, since some fields are modified by the client
	queryArgs := *args
	// whatever happens, the results are expected to be returned in json
	queryArgs.Format = "json"

	var res = new(results.Result)

	req := c.Modify(ctx,
		httpc.NewWithClient("POST", c.NewURL(gqapi.QueryRoute), c.Client()).
			EncodeJSON(queryArgs).
			ParseJSON(res),
	)

	err := req.RunWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run request: %w", err)
	}

	return res, nil
}
