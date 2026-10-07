package server

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

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/api/server"
	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestGenerateOpenAPISpec(t *testing.T) {
	buf := &bytes.Buffer{}
	s := New("localhost:8146", nil, nil)

	err := s.WriteOpenAPISpec(buf)
	require.Nil(t, err)
}

// operationByID returns the operation registered under id in the OpenAPI spec
func operationByID(t *testing.T, oapi *huma.OpenAPI, id string) *huma.Operation {
	t.Helper()
	for _, item := range oapi.Paths {
		for _, op := range []*huma.Operation{item.Get, item.Post, item.Put, item.Patch, item.Delete} {
			if op != nil && op.OperationID == id {
				return op
			}
		}
	}
	t.Fatalf("operation %q not found in spec", id)
	return nil
}

// TestGenerateOpenAPISpec_QueryRoutesDeclareAuthorizationStatuses pins that generated clients
// learn about the statuses the authorizer may answer with on both query routes
func TestGenerateOpenAPISpec_QueryRoutesDeclareAuthorizationStatuses(t *testing.T) {
	oapi := New("localhost:8146", nil, nil).API().OpenAPI()

	for _, id := range []string{"query-post-run", "query-post-run-sse"} {
		t.Run(id, func(t *testing.T) {
			op := operationByID(t, oapi, id)
			for _, status := range []string{"401", "403", "503"} {
				assert.Contains(t, op.Responses, status)
			}
		})
	}
}

// listResolver resolves every query to a fixed host list
type listResolver struct{ out hosts.Hosts }

func (r listResolver) Resolve(_ context.Context, _ string) (hosts.Hosts, error) { return r.out, nil }

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

// principalScope keeps the host IDs listed for the principal
type principalScope struct {
	principal string
	allowed   hosts.Hosts
}

func (s principalScope) Principal() string { return s.principal }

func (s principalScope) Filter(_ context.Context, hostIDs hosts.Hosts) (out hosts.Hosts, err error) {
	for _, id := range hostIDs {
		for _, allowed := range s.allowed {
			if id == allowed {
				out = append(out, id)
			}
		}
	}
	return out, nil
}

// headerAuthorizer is a test authorizer that takes the principal from a request header
type headerAuthorizer struct{ scopes map[string]hosts.Hosts }

func (a headerAuthorizer) Authorize(_ context.Context, req authz.Request) (authz.Scope, error) {
	principal := req.Header("X-Test-Principal")
	if principal == "" {
		return nil, authz.ErrUnauthenticated
	}
	allowed, ok := a.scopes[principal]
	if !ok {
		return nil, authz.ErrForbidden
	}
	return principalScope{principal: principal, allowed: allowed}, nil
}

// startServer serves a global-query API on a unix socket and returns a client for it
func startServer(t *testing.T, querier *recordingQuerier, opts ...server.Option) *http.Client {
	t.Helper()

	dir, err := os.MkdirTemp("", "gq")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "s.sock")

	resolvers := hosts.NewResolverMap()
	resolvers.Set("string", listResolver{out: hosts.Hosts{"h1", "h2", "h3"}})

	srv := New("unix:"+socket, resolvers, querier, opts...)
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

// response is a fully read HTTP response
type response struct {
	status int
	body   []byte
}

// doRequest sends one request to the server behind client and returns the status and the
// fully read body. The body is drained and closed here so that no half-read keep-alive
// connection outlives the test and stalls the server shutdown. A non-empty principal is
// sent in the header the test authorizer reads
func doRequest(t *testing.T, client *http.Client, method, route string, body []byte, principal string) response {
	t.Helper()
	req, err := http.NewRequest(method, "http://unix"+route, bytes.NewReader(body))
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if principal != "" {
		req.Header.Set("X-Test-Principal", principal)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	payload, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return response{status: resp.StatusCode, body: payload}
}

func postQuery(t *testing.T, client *http.Client, principal string) response {
	t.Helper()
	return postQueryTo(t, client, api.QueryRoute, principal)
}

func postQueryTo(t *testing.T, client *http.Client, route, principal string) response {
	t.Helper()
	body, err := json.Marshal(query.Args{Query: "sip", Ifaces: "eth0", Format: "json", QueryHosts: "h1,h2,h3"})
	require.NoError(t, err)
	return doRequest(t, client, http.MethodPost, route, body, principal)
}

func receiveQueried(t *testing.T, querier *recordingQuerier) hosts.Hosts {
	t.Helper()
	select {
	case queried := <-querier.queried:
		return queried
	case <-time.After(5 * time.Second):
		t.Fatal("querier was not called")
		return nil
	}
}

func TestServer_WithAuthorizer_ScopesQueries(t *testing.T) {
	querier := &recordingQuerier{queried: make(chan hosts.Hosts, 1)}
	authorizer := headerAuthorizer{scopes: map[string]hosts.Hosts{
		"alice": {"h2"},
		"bob":   {"elsewhere"},
	}}
	client := startServer(t, querier, server.WithAuthorizer(authorizer))

	t.Run("scoped principal reaches only hosts in scope", func(t *testing.T) {
		resp := postQuery(t, client, "alice")
		require.Equal(t, http.StatusOK, resp.status)
		assert.Equal(t, hosts.Hosts{"h2"}, receiveQueried(t, querier))
	})

	t.Run("principal with no host in query is forbidden", func(t *testing.T) {
		resp := postQuery(t, client, "bob")
		require.Equal(t, http.StatusForbidden, resp.status)
		var problem huma.ErrorModel
		require.NoError(t, json.Unmarshal(resp.body, &problem))
		assert.Equal(t, "no authorized hosts in query", problem.Detail)
		assert.Empty(t, querier.queried)
	})

	t.Run("request without credential is unauthenticated", func(t *testing.T) {
		resp := postQuery(t, client, "")
		require.Equal(t, http.StatusUnauthorized, resp.status)
		assert.Empty(t, querier.queried)
	})
}

// TestServer_RejectedRequests_LeaveRateLimitUntouched pins that authorization runs before the
// rate limiter on both query routes: a burst of rejected requests must not consume the shared
// budget, whereas the same number of entitled requests exhausts it
func TestServer_RejectedRequests_LeaveRateLimitUntouched(t *testing.T) {
	const burst = 2

	for _, route := range []string{api.QueryRoute, api.SSEQueryRoute} {
		t.Run(route, func(t *testing.T) {
			querier := &recordingQuerier{queried: make(chan hosts.Hosts, 1)}
			authorizer := headerAuthorizer{scopes: map[string]hosts.Hosts{"alice": {"h1"}}}
			client := startServer(t, querier,
				server.WithAuthorizer(authorizer),
				// the budget never refills during the test, so only the burst is available
				server.WithQueryRateLimit(rate.Every(time.Hour), burst, 0),
			)

			for i := 0; i < burst; i++ {
				resp := postQueryTo(t, client, route, "")
				require.Equal(t, http.StatusUnauthorized, resp.status, "rejected request %d", i)
			}

			for i := 0; i < burst; i++ {
				resp := postQueryTo(t, client, route, "alice")
				require.Equal(t, http.StatusOK, resp.status, "entitled request %d after rejected burst", i)
				assert.Equal(t, hosts.Hosts{"h1"}, receiveQueried(t, querier))
			}

			resp := postQueryTo(t, client, route, "alice")
			require.Equal(t, http.StatusTooManyRequests, resp.status, "burst must be exhausted by entitled requests only")
			assert.Empty(t, querier.queried)
		})
	}
}

// rejectingAuthorizer rejects every request as unauthenticated
type rejectingAuthorizer struct{}

func (rejectingAuthorizer) Authorize(context.Context, authz.Request) (authz.Scope, error) {
	return nil, authz.ErrUnauthenticated
}

// TestServer_WithAuthorizer_OpenRoutesStayOpen pins that routes which never reach a sensor
// answer without a credential while an authorizer that rejects everything is configured
func TestServer_WithAuthorizer_OpenRoutesStayOpen(t *testing.T) {
	client := startServer(t, &recordingQuerier{queried: make(chan hosts.Hosts, 1)},
		server.WithAuthorizer(rejectingAuthorizer{}),
	)

	validArgs, err := json.Marshal(query.Args{Query: "sip", Ifaces: "eth0", Format: "json", QueryHosts: "h1"})
	require.NoError(t, err)

	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
		status int
	}{
		{name: "health", method: http.MethodGet, path: api.HealthRoute, status: http.StatusOK},
		{name: "ready", method: http.MethodGet, path: api.ReadyRoute, status: http.StatusOK},
		{name: "info", method: http.MethodGet, path: api.InfoRoute, status: http.StatusOK},
		{name: "validate GET", method: http.MethodGet, path: api.ValidationRoute + "?query=sip&ifaces=eth0&format=json&query_hosts=h1", status: http.StatusNoContent},
		{name: "validate POST", method: http.MethodPost, path: api.ValidationRoute, body: validArgs, status: http.StatusNoContent},
		// control: the query route itself is guarded by the same authorizer
		{name: "query POST is rejected", method: http.MethodPost, path: api.QueryRoute, body: validArgs, status: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := doRequest(t, client, tt.method, tt.path, tt.body, "")

			assert.Equal(t, tt.status, resp.status)
		})
	}
}

func TestServer_WithoutAuthorizer_QueriesRunUnscoped(t *testing.T) {
	querier := &recordingQuerier{queried: make(chan hosts.Hosts, 1)}
	client := startServer(t, querier)

	resp := postQuery(t, client, "")
	require.Equal(t, http.StatusOK, resp.status)
	assert.Equal(t, hosts.Hosts{"h1", "h2", "h3"}, receiveQueried(t, querier))
}
