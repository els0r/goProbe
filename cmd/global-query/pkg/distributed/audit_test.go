package distributed

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/telemetry/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logRecord is the structured view of one captured log line
type logRecord struct {
	Level     string   `json:"level"`
	Msg       string   `json:"msg"`
	Principal string   `json:"principal"`
	Requested *int     `json:"requested"`
	Remaining *int     `json:"remaining"`
	Dropped   []string `json:"dropped"`
}

// captureLogs routes the global logger into a buffer as JSON records at the given level
// for the duration of the test and returns a function decoding the records captured so far
func captureLogs(t *testing.T, level slog.Level) func() []logRecord {
	t.Helper()
	buf := &bytes.Buffer{}
	_, err := logging.Init(level, logging.EncodingJSON,
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

// findRecord returns the first record with msg, failing the test when there is none
func findRecord(t *testing.T, records []logRecord, msg string) logRecord {
	t.Helper()
	for _, rec := range records {
		if rec.Msg == msg {
			return rec
		}
	}
	t.Fatalf("no log record with message %q", msg)
	return logRecord{}
}

// auditCtx mirrors what the authorization middleware hands the runner: the scope and a
// context logger carrying the principal
func auditCtx(scope authz.Scope) context.Context {
	ctx := logging.WithFields(context.Background(), slog.String("principal", scope.Principal()))
	return authz.WithScope(ctx, scope)
}

func TestScopedQuery_QueryLogLine_CarriesPrincipal(t *testing.T) {
	records := captureLogs(t, logging.LevelInfo)
	rm := hosts.NewResolverMap()
	rm.Set("string", &mockResolver{out: hosts.Hosts{"h1", "h2", "h3"}})
	qr := NewQueryRunner(rm, &mockQuerier{}, WithScopeEnforcement())

	args := baseArgs()
	args.QueryHosts = "h1,h2,h3"

	_, err := qr.Run(auditCtx(&mockScope{principal: "alice", allowed: []hosts.ID{"h1", "h3"}}), args)
	require.NoError(t, err)

	queryLine := findRecord(t, records(), "reading query results from querier")
	assert.Equal(t, "alice", queryLine.Principal)
}
