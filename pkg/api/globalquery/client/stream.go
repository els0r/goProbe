package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/telemetry/logging"
	jsoniter "github.com/json-iterator/go"
)

type event struct {
	streamType api.StreamEventType
	data       []byte
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
