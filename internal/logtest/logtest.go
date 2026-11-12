// Package logtest is test support for asserting on structured log records. It routes the
// global logger of github.com/els0r/telemetry/logging into a buffer as JSON and decodes the
// records into a caller-defined struct, so that tests assert on fields and never on layout.
// It is not meant for production code
package logtest

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/els0r/telemetry/logging"
	"github.com/stretchr/testify/require"
)

// Capture routes the global logger into a buffer as JSON records at level for the duration
// of the test and returns a function decoding the records captured so far into R, one per
// log line. R is a struct whose json tags name the record fields of interest (for example
// `level`, `msg`, `principal`). The global logger is restored on cleanup
func Capture[R any](t testing.TB, level slog.Level) func() []R {
	t.Helper()
	buf := &bytes.Buffer{}
	_, err := logging.Init(level, logging.EncodingJSON,
		logging.WithOutput(buf), logging.WithErrorOutput(buf))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = logging.Init(logging.LevelDebug, logging.EncodingLogfmt,
			logging.WithOutput(os.Stdout), logging.WithErrorOutput(os.Stderr))
	})
	return func() (records []R) {
		dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
		for dec.More() {
			var rec R
			require.NoError(t, dec.Decode(&rec))
			records = append(records, rec)
		}
		return records
	}
}
