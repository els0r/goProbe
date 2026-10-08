package header_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/plugins/authorizer/header"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRequest is the read-only request view built from an http.Header. EachHeader yields
// every value of every header, as the web framework's context does
type fakeRequest struct{ header http.Header }

func (r fakeRequest) Header(name string) string { return r.header.Get(name) }

func (r fakeRequest) EachHeader(cb func(name, value string)) {
	for name, values := range r.header {
		for _, value := range values {
			cb(name, value)
		}
	}
}

func (r fakeRequest) TLS() *tls.ConnectionState { return nil }

func (r fakeRequest) RemoteAddr() string { return "127.0.0.1:12345" }

// writeConfig writes a header authorizer config file and returns its path
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "header.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestNew_RefusesWithoutTrustedAcknowledgement(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cfgPath string
		wantErr string
	}{
		{name: "empty config path", cfgPath: "", wantErr: "config file required"},
		{name: "trusted missing", cfgPath: writeConfig(t, "scope_header: X-Scope\n"), wantErr: "trusted: true"},
		{name: "trusted false", cfgPath: writeConfig(t, "trusted: false\n"), wantErr: "trusted: true"},
		{name: "config file missing", cfgPath: filepath.Join(t.TempDir(), "none.yaml"), wantErr: "failed to read config"},
		{name: "config not yaml", cfgPath: writeConfig(t, "trusted: [\n"), wantErr: "failed to parse config"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := header.New(tc.cfgPath)
			require.Error(t, err)
			assert.Nil(t, a)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestAuthorize_ScopeHeaderForbidden(t *testing.T) {
	a, err := header.New(writeConfig(t, "trusted: true\n"))
	require.NoError(t, err)

	for _, tc := range []struct {
		name    string
		header  http.Header
		wantErr string
	}{
		{name: "missing", header: http.Header{}, wantErr: "missing"},
		{name: "empty", header: http.Header{"X-Goprobe-Allowed-Hosts": {""}}, wantErr: "empty"},
		{name: "whitespace and commas only", header: http.Header{"X-Goprobe-Allowed-Hosts": {" , ,\t"}}, wantErr: "empty"},
		{name: "repeated", header: http.Header{"X-Goprobe-Allowed-Hosts": {"h1", "h2"}}, wantErr: "present 2 times"},
		{name: "repeated with different case", header: http.Header{
			"X-Goprobe-Allowed-Hosts": {"h1"},
			"x-goprobe-allowed-hosts": {"h2"},
		}, wantErr: "present 2 times"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scope, err := a.Authorize(context.Background(), fakeRequest{header: tc.header})
			require.ErrorIs(t, err, authz.ErrForbidden)
			assert.NotErrorIs(t, err, authz.ErrUnauthenticated)
			assert.ErrorContains(t, err, tc.wantErr)
			assert.Nil(t, scope)
		})
	}
}

func TestScope_FilterMatchesHostIDsExactly(t *testing.T) {
	a, err := header.New(writeConfig(t, "trusted: true\n"))
	require.NoError(t, err)

	for _, tc := range []struct {
		name  string
		scope string
		query hosts.Hosts
		want  hosts.Hosts
	}{
		{name: "literal star matches no host", scope: "*", query: hosts.Hosts{"h1", "h2", "h3"}, want: hosts.Hosts{}},
		{name: "literal star does not widen a scope", scope: "h1,*", query: hosts.Hosts{"h1", "h2", "h3"}, want: hosts.Hosts{"h1"}},
		{name: "trimmed and de-duplicated", scope: " h1 ,h2,, h1 ,\th3 ", query: hosts.Hosts{"h3", "h1", "h2", "h4"}, want: hosts.Hosts{"h3", "h1", "h2"}},
		{name: "exact match is case-sensitive", scope: "H1", query: hosts.Hosts{"h1", "H1"}, want: hosts.Hosts{"H1"}},
		{name: "prefix does not match", scope: "h1", query: hosts.Hosts{"h10", "h1"}, want: hosts.Hosts{"h1"}},
		{name: "nothing in scope", scope: "h9", query: hosts.Hosts{"h1", "h2"}, want: hosts.Hosts{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scope, err := a.Authorize(context.Background(), fakeRequest{header: http.Header{"X-Goprobe-Allowed-Hosts": {tc.scope}}})
			require.NoError(t, err)

			input := append(hosts.Hosts{}, tc.query...)
			got, err := scope.Filter(context.Background(), input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.query, input, "Filter must not mutate its input")
		})
	}
}

func TestAuthorize_Principal(t *testing.T) {
	a, err := header.New(writeConfig(t, "trusted: true\n"))
	require.NoError(t, err)

	// a principal of multi-byte runes, longer than the cap, to prove the cap counts runes
	longPrincipal := strings.Repeat("ä", header.MaxPrincipalLength+10)

	for _, tc := range []struct {
		name   string
		header http.Header
		want   string
	}{
		{name: "present", header: http.Header{
			"X-Goprobe-Allowed-Hosts": {"h1"},
			"X-Goprobe-Principal":     {"alice"},
		}, want: "alice"},
		{name: "absent is unknown", header: http.Header{
			"X-Goprobe-Allowed-Hosts": {"h1"},
		}, want: header.UnknownPrincipal},
		{name: "empty is unknown", header: http.Header{
			"X-Goprobe-Allowed-Hosts": {"h1"},
			"X-Goprobe-Principal":     {""},
		}, want: header.UnknownPrincipal},
		{name: "over-long is capped", header: http.Header{
			"X-Goprobe-Allowed-Hosts": {"h1"},
			"X-Goprobe-Principal":     {longPrincipal},
		}, want: strings.Repeat("ä", header.MaxPrincipalLength)},
		{name: "repeated takes the first value", header: http.Header{
			"X-Goprobe-Allowed-Hosts": {"h1"},
			"X-Goprobe-Principal":     {"alice", "bob"},
		}, want: "alice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scope, err := a.Authorize(context.Background(), fakeRequest{header: tc.header})
			require.NoError(t, err)
			assert.Equal(t, tc.want, scope.Principal())

			got, err := scope.Filter(context.Background(), hosts.Hosts{"h1", "h2"})
			require.NoError(t, err)
			assert.Equal(t, hosts.Hosts{"h1"}, got, "the principal must not change the decision")
		})
	}

	t.Run("principal does not stand in for a missing scope", func(t *testing.T) {
		scope, err := a.Authorize(context.Background(), fakeRequest{header: http.Header{"X-Goprobe-Principal": {"alice"}}})
		require.ErrorIs(t, err, authz.ErrForbidden)
		assert.Nil(t, scope)
	})
}

func TestNew_HeaderNamesAreConfigurable(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		assert.Equal(t, "X-GoProbe-Allowed-Hosts", header.DefaultScopeHeader)
		assert.Equal(t, "X-GoProbe-Principal", header.DefaultPrincipalHeader)
	})

	t.Run("configured names are read, defaults are ignored", func(t *testing.T) {
		a, err := header.New(writeConfig(t, "trusted: true\nscope_header: X-Scope\nprincipal_header: X-Who\n"))
		require.NoError(t, err)

		req := fakeRequest{header: http.Header{
			"X-Scope":                 {"h2"},
			"X-Who":                   {"alice"},
			"X-Goprobe-Allowed-Hosts": {"h1"},
			"X-Goprobe-Principal":     {"mallory"},
		}}
		scope, err := a.Authorize(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "alice", scope.Principal())

		got, err := scope.Filter(context.Background(), hosts.Hosts{"h1", "h2"})
		require.NoError(t, err)
		assert.Equal(t, hosts.Hosts{"h2"}, got)
	})

	t.Run("blank names fall back to the defaults", func(t *testing.T) {
		a, err := header.New(writeConfig(t, "trusted: true\nscope_header: '  '\nprincipal_header: ''\n"))
		require.NoError(t, err)

		req := fakeRequest{header: http.Header{
			"X-Goprobe-Allowed-Hosts": {"h1"},
			"X-Goprobe-Principal":     {"alice"},
		}}
		scope, err := a.Authorize(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "alice", scope.Principal())
	})
}

func TestAuthorize_ScopeFromHeader(t *testing.T) {
	a, err := header.New(writeConfig(t, "trusted: true\n"))
	require.NoError(t, err)

	req := fakeRequest{header: http.Header{}}
	req.header.Set("X-GoProbe-Allowed-Hosts", "h1,h2")

	scope, err := a.Authorize(context.Background(), req)
	require.NoError(t, err)

	got, err := scope.Filter(context.Background(), hosts.Hosts{"h1", "h2", "h3"})
	require.NoError(t, err)
	assert.Equal(t, hosts.Hosts{"h1", "h2"}, got)
}
