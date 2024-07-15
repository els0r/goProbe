package client

import (
	"bytes"
	"context"
	"errors"
	"net/http"

	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/api/client"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/telemetry/logging"
	"github.com/fako1024/httpc"
	jsoniter "github.com/json-iterator/go"
)

// Client denotes a global query client
type Client struct {
	*client.DefaultClient
}

// SSEClient is a global query client capable of streaming updates
type SSEClient struct {
	onUpdate StreamingUpdate
	onFinish StreamingUpdate

	*client.DefaultClient
}

// StreamingUpdate is a function which operates on a received result
type StreamingUpdate func(context.Context, *results.Result) error

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

// NewSSE creates a new streaming client for the global-query API
func NewSSE(addr string, onUpdate, onFinish StreamingUpdate, opts ...client.Option) *SSEClient {
	opts = append(opts, client.WithName(clientName))
	return &SSEClient{
		onUpdate:      onUpdate,
		onFinish:      onFinish,
		DefaultClient: client.NewDefault(addr, opts...),
	}
}

// Run implements the query.Runner interface
func (c *Client) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	return c.Query(ctx, args)
}

// Run implements the query.Runner interface
func (sse *SSEClient) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	return sse.Query(ctx, args)
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

// Query performs the global query and returns its result, while consuming updates to partial results
func (sse *SSEClient) Query(ctx context.Context, args *query.Args) (*results.Result, error) {
	logger := logging.FromContext(ctx)

	if sse.onFinish == nil || sse.onUpdate == nil {
		return nil, errors.New("no event callbacks provided (onUpdate, onFinish)")
	}

	// use a copy of the arguments, since some fields are modified by the client
	queryArgs := *args

	// whatever happens, the results are expected to be returned in json
	queryArgs.Format = "json"

	if queryArgs.Caller == "" {
		queryArgs.Caller = clientName
	}

	buf := &bytes.Buffer{}

	err := jsoniter.NewEncoder(buf).Encode(queryArgs)
	if err != nil {
		return nil, err
	}

	logger.Info("calling SSE route")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sse.NewURL(api.SSEQueryRoute), buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream, application/problem+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := sse.Client().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		return nil, errors.New("no response received")
	}
	defer resp.Body.Close()

	// parse events
	return sse.readEventStream(ctx, resp.Body)
}
