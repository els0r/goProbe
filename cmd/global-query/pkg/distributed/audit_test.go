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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

const msgScopedHostList = "scoped query host list"

func TestScopedQuery_LogsRequestedAndRemainingHostIDCounts(t *testing.T) {
	records := captureLogs(t, logging.LevelInfo)
	rm := hosts.NewResolverMap()
	rm.Set("string", &mockResolver{out: hosts.Hosts{"h1", "h2", "h3"}})
	qr := NewQueryRunner(rm, &mockQuerier{}, WithScopeEnforcement())

	args := baseArgs()
	args.QueryHosts = "h1,h2,h3"

	_, err := qr.Run(auditCtx(&mockScope{principal: "alice", allowed: []hosts.ID{"h1", "h3"}}), args)
	require.NoError(t, err)

	countLine := findRecord(t, records(), msgScopedHostList)
	assert.Equal(t, "info", countLine.Level)
	assert.Equal(t, "alice", countLine.Principal)
	require.NotNil(t, countLine.Requested)
	require.NotNil(t, countLine.Remaining)
	assert.Equal(t, 3, *countLine.Requested)
	assert.Equal(t, 2, *countLine.Remaining)
}

func TestScopedQuery_DuplicateRequestedHostIDs_CountedOnce(t *testing.T) {
	records := captureLogs(t, logging.LevelDebug)
	rm := hosts.NewResolverMap()
	rm.Set("string", &mockResolver{out: hosts.Hosts{"h1", "h1", "h2"}})
	qr := NewQueryRunner(rm, &mockQuerier{}, WithScopeEnforcement())

	args := baseArgs()
	args.QueryHosts = "h1,h1,h2"

	_, err := qr.Run(auditCtx(&mockScope{principal: "alice", allowed: []hosts.ID{"h1", "h2"}}), args)
	require.NoError(t, err)

	countLine := findRecord(t, records(), msgScopedHostList)
	require.NotNil(t, countLine.Requested)
	require.NotNil(t, countLine.Remaining)
	assert.Equal(t, 2, *countLine.Requested)
	assert.Equal(t, 2, *countLine.Remaining)
	assert.Empty(t, droppedHostIDs(records()), "nothing was dropped, so nothing is logged as dropped")
}

// droppedHostIDs collects the dropped host IDs across all captured records
func droppedHostIDs(records []logRecord) (dropped []string) {
	for _, rec := range records {
		dropped = append(dropped, rec.Dropped...)
	}
	return dropped
}

func TestScopedQuery_DroppedHostIDs_LoggedAtDebugOnly(t *testing.T) {
	tests := []struct {
		name    string
		level   slog.Level
		dropped []string
	}{
		{name: "info level omits them", level: logging.LevelInfo, dropped: nil},
		{name: "debug level carries them", level: logging.LevelDebug, dropped: []string{"h2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records := captureLogs(t, tt.level)
			rm := hosts.NewResolverMap()
			rm.Set("string", &mockResolver{out: hosts.Hosts{"h1", "h2", "h3"}})
			qr := NewQueryRunner(rm, &mockQuerier{}, WithScopeEnforcement())

			args := baseArgs()
			args.QueryHosts = "h1,h2,h3"

			_, err := qr.Run(auditCtx(&mockScope{principal: "alice", allowed: []hosts.ID{"h1", "h3"}}), args)
			require.NoError(t, err)

			assert.Equal(t, tt.dropped, droppedHostIDs(records()))
		})
	}
}

func TestUnscopedQuery_NoScopeEnforcement_LogsNoHostIDCounts(t *testing.T) {
	records := captureLogs(t, logging.LevelDebug)
	rm := hosts.NewResolverMap()
	rm.Set("string", &mockResolver{out: hosts.Hosts{"h1", "h2", "h3"}})
	qr := NewQueryRunner(rm, &mockQuerier{})

	args := baseArgs()
	args.QueryHosts = "h1,h2,h3"

	_, err := qr.Run(auditCtx(&mockScope{principal: "alice", allowed: []hosts.ID{"h1"}}), args)
	require.NoError(t, err)

	for _, rec := range records() {
		assert.NotEqual(t, msgScopedHostList, rec.Msg)
		assert.Nil(t, rec.Requested)
		assert.Nil(t, rec.Remaining)
		assert.Empty(t, rec.Dropped)
	}
}

// captureSpans exports every span ended during the test into memory
func captureSpans(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	return exporter
}

// spanAttribute returns the value of the attribute key on the span named name
func spanAttribute(t *testing.T, spans tracetest.SpanStubs, name string, key attribute.Key) string {
	t.Helper()
	for _, span := range spans {
		if span.Name != name {
			continue
		}
		for _, attr := range span.Attributes {
			if attr.Key == key {
				return attr.Value.AsString()
			}
		}
		t.Fatalf("span %q carries no attribute %q", name, key)
	}
	t.Fatalf("no span named %q", name)
	return ""
}

func TestScopedQuery_HostListSpan_CarriesPrincipal(t *testing.T) {
	spans := captureSpans(t)
	rm := hosts.NewResolverMap()
	rm.Set("string", &mockResolver{out: hosts.Hosts{"h1", "h2", "h3"}})
	qr := NewQueryRunner(rm, &mockQuerier{}, WithScopeEnforcement())

	args := baseArgs()
	args.QueryHosts = "h1,h2,h3"

	_, err := qr.Run(auditCtx(&mockScope{principal: "alice", allowed: []hosts.ID{"h1"}}), args)
	require.NoError(t, err)

	principal := spanAttribute(t, spans.GetSpans(), "(*distributed.QueryRunner).prepareHostList", "principal")
	assert.Equal(t, "alice", principal)
}
