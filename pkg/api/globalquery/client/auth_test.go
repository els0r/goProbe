package client_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/v4/pkg/api/client"
	gqclient "github.com/els0r/goProbe/v4/pkg/api/globalquery/client"
	gqserver "github.com/els0r/goProbe/v4/pkg/api/globalquery/server"
	gpclient "github.com/els0r/goProbe/v4/pkg/api/goprobe/client"
	"github.com/els0r/goProbe/v4/pkg/api/server"
	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/pkg/goDB/info"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validToken = "s3cr3t"

// bearerAuthorizer admits requests presenting validToken as a Bearer credential
type bearerAuthorizer struct{}

func (bearerAuthorizer) Authorize(_ context.Context, req authz.Request) (authz.Scope, error) {
	if req.Header("Authorization") != "Bearer "+validToken {
		return nil, authz.ErrUnauthenticated
	}
	return allHosts{}, nil
}

// allHosts is a scope that keeps every host ID
type allHosts struct{}

func (allHosts) Principal() string { return "tester" }

func (allHosts) Filter(_ context.Context, hostIDs hosts.Hosts) (hosts.Hosts, error) {
	return hostIDs, nil
}

// emptyQuerier answers every query without results
type emptyQuerier struct{}

func (emptyQuerier) Query(context.Context, hosts.Hosts, *query.Args) (<-chan *results.Result, <-chan struct{}) {
	rc := make(chan *results.Result)
	close(rc)
	kc := make(chan struct{})
	close(kc)
	return rc, kc
}

type stringResolver struct{}

func (stringResolver) Resolve(_ context.Context, q string) (hosts.Hosts, error) {
	return hosts.Hosts(strings.Split(q, ",")), nil
}

// startScopedServer serves a global-query API protected by bearerAuthorizer and returns the
// address a client is created with, along with the headers of every request received
//
// Client and server share this process and therefore its runtime ID, which the server's
// recursion guard rejects. A reverse proxy stands in for the process boundary: it records
// the request headers as sent by the client, then drops the runtime ID before forwarding
func startScopedServer(t *testing.T) (string, <-chan http.Header) {
	t.Helper()

	dir, err := os.MkdirTemp("", "gq")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "s.sock")
	addr := "unix:" + socket

	resolvers := hosts.NewResolverMap()
	resolvers.Set("string", stringResolver{})

	srv := gqserver.New(addr, resolvers, emptyQuerier{}, server.WithAuthorizer(bearerAuthorizer{}))
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

	received := make(chan http.Header, 16)
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: "unix"})
	proxy.Transport = &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
	}
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- r.Header.Clone():
		default: // tests that don't inspect headers never drain the channel
		}
		r.Header.Del(server.RuntimeIDHeaderKey)
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(front.Close)

	return front.URL, received
}

// newRunner creates a query.Runner of the given kind for addr
type newRunner func(addr string, opts ...client.Option) query.Runner

var runners = map[string]newRunner{
	"query": func(addr string, opts ...client.Option) query.Runner {
		return gqclient.New(addr, opts...)
	},
	"streaming query": func(addr string, opts ...client.Option) query.Runner {
		noUpdate := func(context.Context, *results.Result) error { return nil }
		noKeepalive := func(context.Context) error { return nil }
		return gqclient.NewSSE(addr, noUpdate, noUpdate, noKeepalive, opts...)
	},
}

func runQuery(ctx context.Context, runner query.Runner) (*results.Result, error) {
	return runner.Run(ctx, &query.Args{Query: "sip", Ifaces: "eth0", QueryHosts: "h1", Format: "json"})
}

func requireStatus(t *testing.T, err error, status int) {
	t.Helper()
	var statusErr huma.StatusError
	require.ErrorAs(t, err, &statusErr, "error %v carries no status", err)
	assert.Equal(t, status, statusErr.GetStatus())
}

func TestClients_BearerToken_AuthenticatesScopedQueries(t *testing.T) {
	addr, _ := startScopedServer(t)

	for name, newRunner := range runners {
		t.Run(name, func(t *testing.T) {
			t.Run("with token", func(t *testing.T) {
				res, err := runQuery(context.Background(), newRunner(addr, client.WithBearerToken(validToken)))
				require.NoError(t, err)
				assert.NotNil(t, res)
			})
			t.Run("without token", func(t *testing.T) {
				_, err := runQuery(context.Background(), newRunner(addr))
				require.Error(t, err)
				if name == "streaming query" {
					requireStatus(t, err, http.StatusUnauthorized)
				}
			})
		})
	}
}

type tokenKey struct{}

// contextTokenTransport forwards the Bearer token found in each request's context, as a
// service relaying its own caller's credential would
type contextTokenTransport struct{ next http.RoundTripper }

func (c contextTokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	token, ok := r.Context().Value(tokenKey{}).(string)
	if !ok {
		return c.next.RoundTrip(r)
	}
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+token)
	return c.next.RoundTrip(r)
}

func TestClients_WithTransport_ForwardsTokenFromRequestContext(t *testing.T) {
	addr, _ := startScopedServer(t)
	transport := client.WithTransport(contextTokenTransport{next: http.DefaultTransport})

	for name, newRunner := range runners {
		t.Run(name, func(t *testing.T) {
			runner := newRunner(addr, transport)

			ctx := context.WithValue(context.Background(), tokenKey{}, validToken)
			_, err := runQuery(ctx, runner)
			require.NoError(t, err)

			ctx = context.WithValue(context.Background(), tokenKey{}, "wrong")
			_, err = runQuery(ctx, runner)
			require.Error(t, err)
		})
	}
}

// TestClients_SendIdenticalHeaders pins that the streaming route carries the same runtime ID
// and credential as the synchronous one, including the legacy digest key
func TestClients_SendIdenticalHeaders(t *testing.T) {
	addr, received := startScopedServer(t)

	for name, newRunner := range runners {
		t.Run(name, func(t *testing.T) {
			_, err := runQuery(context.Background(), newRunner(addr, client.WithAPIKey("legacy")))
			require.Error(t, err, "a digest key is not a Bearer credential")

			headers := <-received
			assert.Equal(t, "digest legacy", headers.Get("Authorization"))
			assert.Equal(t, info.RuntimeID(), headers.Get(server.RuntimeIDHeaderKey))
		})
	}
}

func TestNewSSEFromConfig_Token_AuthenticatesScopedQueries(t *testing.T) {
	addr, _ := startScopedServer(t)

	runner := gqclient.NewSSEFromConfig(&gpclient.Config{Addr: addr, Key: "legacy", Token: validToken}, nil)
	_, err := runQuery(context.Background(), runner)
	require.NoError(t, err, "the token takes precedence over the key")
}
