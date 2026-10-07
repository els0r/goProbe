package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/els0r/goProbe/v4/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/distributed"
	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/plugins"
	"github.com/els0r/goProbe/v4/plugins/resolver/stringresolver"
	"github.com/els0r/telemetry/logging"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testQuerierType    = "test-recording"
	testAuthorizerType = "test-header-scope"
	testPrincipalHdr   = "X-Test-Principal"
)

// recordingQuerier records the host IDs it is asked to query and answers nothing
type recordingQuerier struct{ queried chan hosts.Hosts }

func (q *recordingQuerier) Query(_ context.Context, queryHosts hosts.Hosts, _ *query.Args) (<-chan *results.Result, <-chan struct{}) {
	q.queried <- queryHosts
	rc := make(chan *results.Result)
	close(rc)
	kc := make(chan struct{})
	close(kc)
	return rc, kc
}

// testScope keeps the single host ID h2 for its principal
type testScope struct{ principal string }

func (s testScope) Principal() string { return s.principal }

func (s testScope) Filter(_ context.Context, hostIDs hosts.Hosts) (out hosts.Hosts, err error) {
	for _, id := range hostIDs {
		if id == "h2" {
			out = append(out, id)
		}
	}
	return out, nil
}

// testAuthorizer scopes the principal named in a request header to h2
type testAuthorizer struct{}

func (testAuthorizer) Authorize(_ context.Context, req authz.Request) (authz.Scope, error) {
	principal := req.Header(testPrincipalHdr)
	if principal == "" {
		return nil, authz.ErrUnauthenticated
	}
	return testScope{principal: principal}, nil
}

// testQuerier is the querier the test plugin hands out, so tests can observe what was queried
var testQuerier = &recordingQuerier{queried: make(chan hosts.Hosts, 1)}

// the test plugins register exactly like in-tree or contrib plugins do: from init()
func init() {
	plugins.RegisterQuerier(testQuerierType, func(_ context.Context, _ string) (distributed.Querier, error) {
		return testQuerier, nil
	})
	plugins.RegisterAuthorizer(testAuthorizerType, func(_ context.Context, _ string) (authz.Authorizer, error) {
		return testAuthorizer{}, nil
	})
}

// configureServer sets the viper keys a server start reads, selecting the test querier and
// the given authorizer type
func configureServer(t *testing.T, authorizerType string) {
	t.Helper()
	viper.Reset()
	viper.Set("hosts.resolvers", []map[string]string{})
	viper.Set(conf.HostsResolverType, stringresolver.Type)
	viper.Set(conf.QuerierType, testQuerierType)
	viper.Set(conf.AuthorizerType, authorizerType)
}

// startConfiguredServer builds the server from the viper config the way the server command
// does, serves it on a unix socket and returns a client for it
func startConfiguredServer(t *testing.T) *http.Client {
	t.Helper()

	// a short directory: t.TempDir embeds the test name, which exceeds the unix socket path limit
	dir, err := os.MkdirTemp("", "gq")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "s.sock")

	srv, err := newAPIServer(context.Background(), "unix:"+socket)
	require.NoError(t, err)
	go func() {
		if err := srv.Serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("serve: %v", err)
		}
	}()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	require.Eventually(t, func() bool {
		conn, err := net.Dial("unix", socket)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 5*time.Second, 10*time.Millisecond)

	return &http.Client{Transport: &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return net.Dial("unix", socket)
		},
	}}
}

// doQuery posts a query for h1,h2,h3 as the given principal (none if empty). It drains and
// closes the response body before returning, so the server can shut down without stalling
func doQuery(t *testing.T, client *http.Client, principal string) (status int, body []byte) {
	t.Helper()
	args, err := json.Marshal(query.Args{Query: "sip", Ifaces: "eth0", Format: "json", QueryHosts: "h1,h2,h3"})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, "http://unix"+api.QueryRoute, bytes.NewReader(args))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if principal != "" {
		req.Header.Set(testPrincipalHdr, principal)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode, body
}

func receiveQueried(t *testing.T) hosts.Hosts {
	t.Helper()
	select {
	case queried := <-testQuerier.queried:
		return queried
	case <-time.After(5 * time.Second):
		t.Fatal("querier was not called")
		return nil
	}
}

// logRecord is the structured view of one captured log line
type logRecord struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// captureLogs routes the global logger into a buffer as JSON records for the duration of the
// test and returns a function decoding the records captured so far
func captureLogs(t *testing.T) func() []logRecord {
	t.Helper()
	buf := &bytes.Buffer{}
	_, err := logging.Init(logging.LevelDebug, logging.EncodingJSON,
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

// countUnscopedWarnings counts the warning records announcing unscoped operation
func countUnscopedWarnings(records []logRecord) (n int) {
	for _, rec := range records {
		if rec.Level == "warn" && rec.Msg == msgUnscopedQueries {
			n++
		}
	}
	return n
}

func TestServerCommand_AuthorizerFlags(t *testing.T) {
	viper.Reset()
	cmd, err := serverCommand()
	require.NoError(t, err)

	for _, tc := range []struct {
		name string
		key  string
		def  string
	}{
		{name: "authorizer type flag", key: conf.AuthorizerType, def: conf.DefaultAuthorizerType},
		{name: "authorizer config flag", key: conf.AuthorizerConfig, def: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flag := cmd.PersistentFlags().Lookup(tc.key)
			require.NotNil(t, flag, "flag --%s", tc.key)
			assert.Equal(t, tc.def, flag.DefValue)
		})
	}
}

func TestServer_NoAuthorizer_WarnsOnceAtStartup(t *testing.T) {
	t.Run("empty type warns once that queries run unscoped", func(t *testing.T) {
		configureServer(t, "")
		records := captureLogs(t)

		_, err := newAPIServer(context.Background(), "localhost:0")
		require.NoError(t, err)

		assert.Equal(t, 1, countUnscopedWarnings(records()), "records: %+v", records())
	})

	t.Run("configured authorizer does not warn", func(t *testing.T) {
		configureServer(t, testAuthorizerType)
		records := captureLogs(t)

		_, err := newAPIServer(context.Background(), "localhost:0")
		require.NoError(t, err)

		assert.Equal(t, 0, countUnscopedWarnings(records()), "records: %+v", records())
	})
}

func TestServer_ConfiguredAuthorizer_ScopesQueriesOverHTTP(t *testing.T) {
	configureServer(t, testAuthorizerType)
	client := startConfiguredServer(t)

	t.Run("scoped principal reaches only hosts in scope", func(t *testing.T) {
		status, _ := doQuery(t, client, "alice")
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, hosts.Hosts{"h2"}, receiveQueried(t))
	})

	t.Run("request without credential is unauthenticated", func(t *testing.T) {
		status, body := doQuery(t, client, "")
		require.Equal(t, http.StatusUnauthorized, status, "body: %s", body)
		assert.Empty(t, testQuerier.queried)
	})
}
