package client

import (
	"bytes"
	"context"
	"net/http"

	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/api/client"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/fako1024/httpc"
	jsoniter "github.com/json-iterator/go"
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
		httpc.NewWithClient(http.MethodPost, c.NewURL(api.QueryRoute), c.Client()).
			EncodeJSON(queryArgs).
			ParseJSON(res),
	)

	err := req.RunWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// QuerySSE performs the global query and returns its result, while consuming updates
// to partial results
func (c *Client) QuerySSE(ctx context.Context, args *query.Args, onUpdate, onFinish func(*results.Result) error) (*results.Result, error) {
	// use a copy of the arguments, since some fields are modified by the client
	queryArgs := *args

	// whatever happens, the results are expected to be returned in json
	queryArgs.Format = "json"

	if queryArgs.Caller == "" {
		queryArgs.Caller = clientName
	}

	var res = new(results.Result)

	buf := &bytes.Buffer{}

	err := jsoniter.NewEncoder(buf).Encode(queryArgs)
	if err != nil {
		return nil, err
	}

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.NewURL(api.SSEQueryRoute), buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	resp, err := c.Client().Do(req)
	if err != nil {
		return nil, err
	}

	// parse events

	return res, nil
}
