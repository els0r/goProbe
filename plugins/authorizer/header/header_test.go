package header_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
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
