package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/els0r/goProbe/v4/internal/logtest"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/telemetry/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQueryFailure_LogLevel pins the level of the one line the query handlers log for a
// failed query: a 4xx is an outcome of the request and a warning, including the 422 of
// invalid query arguments; a 5xx and an error without a status are errors of the server
func TestQueryFailure_LogLevel(t *testing.T) {
	type logRecord struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}

	validArgs := query.Args{Query: "sip", Ifaces: "eth0", Format: "json", QueryHosts: "h1"}
	// passes the schema validation of the request body and fails when the query is prepared
	invalidArgs := validArgs
	invalidArgs.Condition = "not a condition"

	tests := []struct {
		name      string
		args      query.Args
		runnerErr error
		level     string
	}{
		{name: "422 invalid query args", args: invalidArgs, level: "warn"},
		{
			name:      "403 from the runner",
			args:      validArgs,
			runnerErr: query.NewDetailError(http.StatusForbidden, errors.New("no authorized hosts in query")),
			level:     "warn",
		},
		{
			name:      "500 from the runner",
			args:      validArgs,
			runnerErr: query.NewDetailError(http.StatusInternalServerError, errors.New("internal")),
			level:     "error",
		},
		{
			name:      "503 from the runner",
			args:      validArgs,
			runnerErr: query.NewDetailError(http.StatusServiceUnavailable, errors.New("authorization unavailable")),
			level:     "error",
		},
		{name: "error without a status", args: validArgs, runnerErr: errors.New("querier failed"), level: "error"},
	}

	routes := map[string]string{
		QueryRoute:    "error running plain query",
		SSEQueryRoute: "error running SSE query",
	}

	for _, tt := range tests {
		for route, msg := range routes {
			t.Run(tt.name+" "+route, func(t *testing.T) {
				records := logtest.Capture[logRecord](t, logging.LevelInfo)
				api := setupAPI(t, failingRunner{err: tt.runnerErr})

				api.Post(route, "Content-Type: application/json", tt.args)

				var failures []logRecord
				for _, rec := range records() {
					if rec.Msg == msg {
						failures = append(failures, rec)
					}
				}
				require.Len(t, failures, 1, "exactly one %q line", msg)
				assert.Equal(t, tt.level, failures[0].Level)
			})
		}
	}
}
