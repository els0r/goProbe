package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/api/client"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/telemetry/logging"
	jsoniter "github.com/json-iterator/go"
)

type event struct {
	streamType api.StreamEventType
	data       []byte
}

// SSEClient is a global query client capable of streaming updates
type SSEClient struct {
	onUpdate StreamingUpdate
	onFinish StreamingUpdate

	*client.DefaultClient
}

// StreamingUpdate is a function which operates on a received result
type StreamingUpdate func(context.Context, *results.Result) error

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

	// parse events
	return sse.readEventStream(ctx, resp.Body)
}

func (sse *SSEClient) readEventStream(ctx context.Context, r io.Reader) (res *results.Result, err error) {
	logger := logging.FromContext(ctx)

	reader := bufio.NewReader(r)
	res = new(results.Result)

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

			logger.With("event_type", event.streamType, "events_received", eventsReceived).Debug("received event")

			switch event.streamType {
			case api.StreamEventQueryError:
				// we know that this is a query.DetailError
				var qe = &query.DetailError{}
				if err := jsoniter.Unmarshal(event.data, qe); err != nil {
					// if the detail error couldn't be parsed, return error as is
					return nil, errors.New(string(event.data))
				}
				return nil, qe
			case api.StreamEventPartialResult, api.StreamEventFinalResult:
				if err := jsoniter.Unmarshal(event.data, res); err != nil {
					logger.With("error", err).Error("failed to parse JSON")
					continue
				}
				// exit streaming if this is the final result
				if event.streamType == api.StreamEventFinalResult {
					return res, sse.onFinish(ctx, res)
				}

				if err := sse.onUpdate(ctx, res); err != nil {
					logger.With("error", err).Error("failed to call update callback")
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
)
