package client

import (
	"context"

	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/fako1024/httpc"
)

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

	if queryArgs.Caller == "" {
		queryArgs.Caller = clientName
	}

	// we need more results before truncating
	if queryArgs.NumResults < query.DefaultNumResults {
		queryArgs.NumResults = query.DefaultNumResults
	}

	var res = new(results.Result)

	req := c.Modify(ctx,
		httpc.NewWithClient("POST", c.NewURL(gpapi.QueryRoute), c.Client()).
			EncodeJSON(queryArgs).
			ParseJSON(res),
	)
	err := req.RunWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// Query runs a query on the API endpoint
func (c *Client) Validate(ctx context.Context, args *query.Args) (*results.Result, error) {
	var res = new(results.Result)

	req := c.Modify(ctx,
		httpc.NewWithClient("POST", c.NewURL(gpapi.ValidationRoute), c.Client()).
			EncodeJSON(args).
			ParseJSON(res),
	)
	err := req.RunWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return res, nil
}
