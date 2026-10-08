package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/api/client"
	gpclient "github.com/els0r/goProbe/v4/pkg/api/goprobe/client"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/telemetry/logging"
	jsoniter "github.com/json-iterator/go"
)

type event struct {
	streamType api.StreamEventType
	data       []byte
}

// SSEClient is a global query client capable of streaming updates
type SSEClient struct {
	onUpdate    StreamingUpdate
	onFinish    StreamingUpdate
	onKeepalive StreamingKeepalive

	*client.DefaultClient
}

// StreamingUpdate is a function which operates on a received result
type StreamingUpdate func(context.Context, *results.Result) error

// StreamingKeepalive is a function which operates on a received keepalive
type StreamingKeepalive func(context.Context) error

// NewSSE creates a new streaming client for the global-query API
func NewSSE(addr string, onUpdate, onFinish StreamingUpdate, onKeepalive StreamingKeepalive, opts ...client.Option) *SSEClient {
	opts = append(opts, client.WithName(clientName))
	return &SSEClient{
		onUpdate:      onUpdate,
		onFinish:      onFinish,
		onKeepalive:   onKeepalive,
		DefaultClient: client.NewDefault(addr, opts...),
	}
}

// NewFromConfig creates the client based on cfg, sending keepalive signals on
// the provided channel
func NewSSEFromConfig(cfg *gpclient.Config, keepaliveChan chan<- struct{}) *SSEClient {
	return NewSSE(cfg.Addr,
		// TODO: this will become more informational in the future as in: printing partial results, etc.
		func(ctx context.Context, r *results.Result) error {
			if r == nil {
				return nil
			}
			all := len(r.HostsStatuses)
			errs := len(r.HostsStatuses.GetErrorStatuses())

			logger := logging.FromContext(ctx)
			logger.Info("received update", "total", all, "done", all-errs, "errors", errs)

			return nil
		},
		func(ctx context.Context, r *results.Result) error { return nil },
		func(ctx context.Context) error {
			if keepaliveChan == nil {
				return nil
			}
			select {
			case keepaliveChan <- struct{}{}:
			default:
			}

			return nil
		},
		client.WithRequestTimeout(cfg.RequestTimeout),
		client.WithScheme(cfg.Scheme),
		client.WithAPIKey(cfg.Key),
	)
}

// Run implements the query.Runner interface
func (sse *SSEClient) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	return sse.Query(ctx, args)
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
	queryArgs.Format = types.FormatJSON

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

	// a rejection before the stream opens (e.g. by the authorization middleware) is answered
	// instead of an event stream
	if resp.StatusCode/100 != 2 {
		return nil, responseError(resp)
	}

	// parse events
	return sse.readEventStream(ctx, resp.Body)
}

// responseError turns a response the server sent instead of an event stream into an error
// that carries the response's status. An RFC 9457 problem body is returned as is; any
// other body becomes the detail
func responseError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read body of %s response: %w", resp.Status, err)
	}

	problem := new(query.DetailError)
	if err := jsoniter.Unmarshal(body, problem); err == nil && problem.Status != 0 {
		return problem
	}

	detail := string(bytes.TrimSpace(body))
	if detail == "" {
		detail = resp.Status
	}
	return query.NewDetailError(resp.StatusCode, errors.New(detail))
}

func (sse *SSEClient) readEventStream(ctx context.Context, r io.Reader) (res *results.Result, err error) {
	logger := logging.FromContext(ctx)

	reader := bufio.NewReader(r)
	res = new(results.Result)
	keepalive := new(api.Keepalive)

	var eventsReceived int
	for {
		select {
		case <-ctx.Done():
			logger.Info("request cancelled")
			return res, nil
		default:
			event, err := readEvent(reader)
			if err != nil {
				if errors.Is(err, io.EOF) {
					logger.Info("stream finished")
					return res, nil
				}
				return nil, fmt.Errorf("failed to read SSE event: %w", err)
			}
			eventsReceived++

			logger.Info("received event", "event_type", event.streamType, "events_received", eventsReceived)

			switch event.streamType {
			case api.StreamEventQueryError:
				// we know that this is a query.DetailError
				var qe = &query.DetailError{}
				if err := jsoniter.Unmarshal(event.data, qe); err != nil {
					// if the detail error couldn't be parsed, return error as is
					return nil, errors.New(string(event.data))
				}
				return nil, qe
			case api.StreamEventKeepalive:
				if err := jsoniter.Unmarshal(event.data, keepalive); err != nil {
					logger.Error("failed to parse JSON", "error", err)
					continue
				}
				if err := sse.onKeepalive(ctx); err != nil {
					logger.Error("failed to call keepalive callback", "error", err)
				}
			case api.StreamEventPartialResult, api.StreamEventFinalResult:
				if err := jsoniter.Unmarshal(event.data, res); err != nil {
					logger.Error("failed to parse JSON", "error", err)
					continue
				}
				// exit streaming if this is the final result
				if event.streamType == api.StreamEventFinalResult {
					return res, sse.onFinish(ctx, res)
				}

				if err := sse.onUpdate(ctx, res); err != nil {
					logger.Error("failed to call update callback", "error", err)
				}
			}
		}
	}
}

func readEvent(r *bufio.Reader) (*event, error) {
	event := &event{}

	// consume all empty lines or newline lines
	// TODO: Maybe this can be improved in the future (e.g. using a bufio.Scanner or similar)
	var line []byte
	var err error
	for len(line) == 0 || string(line) == "\n" {
		line, err = r.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read SSE stream: %w", err)
		}
	}

	if bytes.HasPrefix(line, eventPrefix) {
		bytesSpl := bytes.Split(line, eventPrefix)
		if len(bytesSpl) < 2 {
			return nil, errors.New("event: malformed data")
		}
		data := bytes.TrimRight(bytesSpl[1], "\n")

		switch {
		case bytes.Equal(data, queryError):
			event.streamType = api.StreamEventQueryError
		case bytes.Equal(data, partialResult):
			event.streamType = api.StreamEventPartialResult
		case bytes.Equal(data, finalResult):
			event.streamType = api.StreamEventFinalResult
		case bytes.Equal(data, keepalive):
			event.streamType = api.StreamEventKeepalive
			// TODO: default case required?
		}
	}

	// advance the reader to read the next line
	line, err = r.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read 'data' key off SSE stream: %w", err)
	}

	if bytes.HasPrefix(line, dataPrefix) {
		bytesSpl := bytes.Split(line, dataPrefix)
		if len(bytesSpl) < 2 {
			return nil, errors.New("data: malformed data")
		}
		event.data = bytes.TrimRight(bytesSpl[1], "\n")
	}

	return event, nil
}

var (
	eventPrefix = []byte("event: ")
	dataPrefix  = []byte("data: ")

	queryError    = []byte(api.StreamEventQueryError)
	partialResult = []byte(api.StreamEventPartialResult)
	finalResult   = []byte(api.StreamEventFinalResult)
	keepalive     = []byte(api.StreamEventKeepalive)
)
