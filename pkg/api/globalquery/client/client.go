package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
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

// QuerySSE performs the global query and returns its result, while consuming updates to partial results
func (c *Client) QuerySSE(ctx context.Context, args *query.Args, onUpdate, onFinish func(context.Context, *results.Result) error) (*results.Result, error) {
	logger := logging.FromContext(ctx)

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
	if resp.Body == nil {
		return nil, errors.New("no response received")
	}
	defer resp.Body.Close()

	// parse events
	var eventType api.StreamEventType
	var eventsReceived int

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			logger.Info("request cancelled")
			return res, nil
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return nil, fmt.Errorf("failed to read SSE stream: %w", err)
			}

			if len(line) == 0 || string(line) == "\n" {
				continue
			}

			switch {
			case bytes.HasPrefix(line, eventPrefix):
				bytesSpl := bytes.Split(line, eventPrefix)
				if len(bytesSpl) < 2 {
					continue
				}
				data := bytesSpl[1]

				switch {
				case bytes.Equal(data, queryError):
					eventType = api.StreamEventQueryError
				case bytes.Equal(data, partialResult):
					eventType = api.StreamEventPartialResult
				case bytes.Equal(data, finalResult):
					eventType = api.StreamEventFinalResult
				}
				eventsReceived++

				logger.With("event_type", eventType, "events_received", eventsReceived).Debug("received event")
				// get to the data
				continue
			case bytes.HasPrefix(line, dataPrefix):
				bytesSpl := bytes.Split(line, dataPrefix)
				if len(bytesSpl) < 2 {
					continue
				}
				data := bytesSpl[1]

				switch eventType {
				case api.StreamEventQueryError:
					return nil, errors.New(string(data))
				case api.StreamEventPartialResult, api.StreamEventFinalResult:
					// parse the results data
					var res = new(results.Result)
					if err := jsoniter.Unmarshal(data, res); err != nil {
						logger.Error("failed to parse JSON", "error", err)
						continue
					}

					// exit streaming if this is the final result
					if eventType == api.StreamEventFinalResult {
						return res, onFinish(ctx, res)
					}

					if err := onUpdate(ctx, res); err != nil {
						logger.Error("failed to call update callback", "error", err)
					}
				default:
					continue
				}
			default:
				continue
			}
		}
	}

	return res, nil
}

var (
	eventPrefix = []byte("event:")
	dataPrefix  = []byte("data:")

	queryError    = []byte(api.StreamEventQueryError)
	partialResult = []byte(api.StreamEventPartialResult)
	finalResult   = []byte(api.StreamEventFinalResult)
)
