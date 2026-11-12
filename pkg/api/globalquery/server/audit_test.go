package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/els0r/goProbe/v4/pkg/api/server"
	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/telemetry/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	msgAccessLog  = "handled request"
	msgQueryLine  = "reading query results from querier"
	msgHostCounts = "scoped query host list"
)

// logRecord is the structured view of one captured log line
type logRecord struct {
	Level     string `json:"level"`
	Msg       string `json:"msg"`
	Principal string `json:"principal"`
	Error     string `json:"error"`
}

// captureLogs routes the global logger into a buffer as JSON records for the duration of the
// test and returns a function decoding the records captured so far
func captureLogs(t *testing.T) func() []logRecord {
	t.Helper()
	buf := &bytes.Buffer{}
	_, err := logging.Init(logging.LevelInfo, logging.EncodingJSON,
		logging.WithOutput(buf), logging.WithErrorOutput(buf))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = logging.Init(logging.LevelDebug, logging.EncodingLogfmt,
			logging.WithOutput(os.Stdout), logging.WithErrorOutput(os.Stderr))
	})
	return func() (records []logRecord) {
		dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
		for dec.More() {
			var rec logRecord
			require.NoError(t, dec.Decode(&rec))
			records = append(records, rec)
		}
		return records
	}
}

// recordsWithLevel returns the records at level
func recordsWithLevel(records []logRecord, level string) (out []logRecord) {
	for _, rec := range records {
		if rec.Level == level {
			out = append(out, rec)
		}
	}
	return out
}

// awaitRecord waits for the record with msg: the access-log line is written after the
// handler chain returns, which may be after the client has read the response
func awaitRecord(t *testing.T, records func() []logRecord, msg string) logRecord {
	t.Helper()
	var found logRecord
	require.Eventually(t, func() bool {
		for _, rec := range records() {
			if rec.Msg == msg {
				found = rec
				return true
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond, "no log record with message %q", msg)
	return found
}

// reasonAuthorizer rejects every request with a reason only the operator may see
type reasonAuthorizer struct{}

const rejectionReason = "token expired at provider"

func (reasonAuthorizer) Authorize(context.Context, authz.Request) (authz.Scope, error) {
	return nil, fmt.Errorf("%s: %w", rejectionReason, authz.ErrUnauthenticated)
}

func TestServer_RejectedRequest_OneWarningWithReason_NotReturned(t *testing.T) {
	records := captureLogs(t)
	querier := &recordingQuerier{queried: make(chan hosts.Hosts, 1)}
	client := startServer(t, querier, server.WithAuthorizer(reasonAuthorizer{}))

	resp := postQuery(t, client, "alice")

	require.Equal(t, http.StatusUnauthorized, resp.status)
	assert.NotContains(t, string(resp.body), rejectionReason)

	accessLog := awaitRecord(t, records, msgAccessLog)
	assert.Empty(t, accessLog.Principal, "the access-log line does not carry the principal")

	warnings := recordsWithLevel(records(), "warn")
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Error, rejectionReason)
	for _, rec := range recordsWithLevel(records(), "error") {
		assert.Equal(t, msgAccessLog, rec.Msg, "only the access-log line of a 4xx is at error level")
	}
	assert.Empty(t, querier.queried)
}
