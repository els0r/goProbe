package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingRunner fails every query with a preset error
type failingRunner struct{ err error }

func (r failingRunner) Run(context.Context, *query.Args) (*results.Result, error) {
	return nil, r.err
}

func (r failingRunner) RunStreaming(context.Context, *query.Args, sse.Sender) (*results.Result, error) {
	return nil, r.err
}

// statusWithCause mirrors the runner's outcome errors: an HTTP status and fixed detail with a
// wrapped cause that must never reach the client
type statusWithCause struct {
	*query.DetailError
	cause error
}

func (e *statusWithCause) Unwrap() error { return e.cause }

// readQueryErrorEvent returns the data of the first queryError event of an SSE body
func readQueryErrorEvent(t *testing.T, body string) query.DetailError {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(body))
	inQueryError := false
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "event: "+string(StreamEventQueryError):
			inQueryError = true
		case inQueryError && strings.HasPrefix(line, "data: "):
			var detail query.DetailError
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &detail))
			return detail
		}
	}
	t.Fatalf("no %s event in body:\n%s", StreamEventQueryError, body)
	return query.DetailError{}
}

func TestStreamingQuery_RunnerErrorStatus_PreservedInQueryErrorEvent(t *testing.T) {
	cause := errors.New("secret cause")

	tests := []struct {
		name   string
		err    error
		status int
		detail string
	}{
		{
			name: "forbidden",
			err: &statusWithCause{
				DetailError: query.NewDetailError(http.StatusForbidden, errors.New("no authorized hosts in query")),
				cause:       cause,
			},
			status: http.StatusForbidden,
			detail: "no authorized hosts in query",
		},
		{
			name: "unavailable",
			err: &statusWithCause{
				DetailError: query.NewDetailError(http.StatusServiceUnavailable, errors.New("authorization unavailable")),
				cause:       cause,
			},
			status: http.StatusServiceUnavailable,
			detail: "authorization unavailable",
		},
		{
			name:   "error without status",
			err:    errors.New("querier exploded"),
			status: http.StatusInternalServerError,
			detail: "querier exploded",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, api := humatest.New(t)
			RegisterQueryAPI(api, "test", failingRunner{err: tt.err}, nil)

			resp := api.Post(SSEQueryRoute, distributedArgs())
			require.Equal(t, http.StatusOK, resp.Code)

			event := readQueryErrorEvent(t, resp.Body.String())
			assert.Equal(t, tt.status, event.Status)
			assert.Equal(t, tt.detail, event.Detail)
			assert.NotContains(t, resp.Body.String(), "secret cause")
		})
	}
}
