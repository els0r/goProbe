package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubScope is a Scope with a fixed principal that keeps every host ID
type stubScope struct{ principal string }

func (s stubScope) Principal() string { return s.principal }

func (stubScope) Filter(_ context.Context, hostIDs hosts.Hosts) (hosts.Hosts, error) {
	return hostIDs, nil
}

// stubAuthorizer answers every request with a preset scope or error
type stubAuthorizer struct {
	scope authz.Scope
	err   error
}

func (a stubAuthorizer) Authorize(_ context.Context, _ authz.Request) (authz.Scope, error) {
	return a.scope, a.err
}

// scopeRecordingRunner records the scope a query arrived with
type scopeRecordingRunner struct {
	calls      int
	principals []string
}

func (r *scopeRecordingRunner) record(ctx context.Context) {
	r.calls++
	if scope, ok := authz.ScopeFromContext(ctx); ok {
		r.principals = append(r.principals, scope.Principal())
	}
}

func (r *scopeRecordingRunner) Run(ctx context.Context, _ *query.Args) (*results.Result, error) {
	r.record(ctx)
	return results.New(), nil
}

func (r *scopeRecordingRunner) RunStreaming(ctx context.Context, _ *query.Args, _ sse.Sender) (*results.Result, error) {
	r.record(ctx)
	return results.New(), nil
}

func setupAuthorizedAPI(t *testing.T, authorizer authz.Authorizer, runner SSEQueryRunner) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterQueryAPI(api, "test", runner, huma.Middlewares{AuthorizationMiddleware(api, authorizer)})
	return api
}

func distributedArgs() query.Args {
	return query.Args{Query: "sip", Ifaces: "eth0", Format: "json", QueryHosts: "hostA"}
}

func decodeProblem(t *testing.T, body []byte) huma.ErrorModel {
	t.Helper()
	var problem huma.ErrorModel
	require.NoError(t, json.Unmarshal(body, &problem))
	return problem
}

func TestAuthorization_RejectedRequest_StatusMapping(t *testing.T) {
	const secret = "ldap://provider: bind failed with secret detail"

	tests := []struct {
		name       string
		authorizer stubAuthorizer
		status     int
		detail     string
	}{
		{
			name:       "unauthenticated",
			authorizer: stubAuthorizer{err: fmt.Errorf("%s: %w", secret, authz.ErrUnauthenticated)},
			status:     http.StatusUnauthorized,
			detail:     "unauthenticated",
		},
		{
			name:       "forbidden",
			authorizer: stubAuthorizer{err: fmt.Errorf("%s: %w", secret, authz.ErrForbidden)},
			status:     http.StatusForbidden,
			detail:     "forbidden",
		},
		{
			name:       "authorizer failure",
			authorizer: stubAuthorizer{err: errors.New(secret)},
			status:     http.StatusServiceUnavailable,
			detail:     "authorization unavailable",
		},
		{
			name:       "authorizer returns neither scope nor error",
			authorizer: stubAuthorizer{},
			status:     http.StatusServiceUnavailable,
			detail:     "authorization unavailable",
		},
	}
	for _, tt := range tests {
		for _, route := range []string{QueryRoute, SSEQueryRoute} {
			t.Run(tt.name+" "+route, func(t *testing.T) {
				runner := &scopeRecordingRunner{}
				api := setupAuthorizedAPI(t, tt.authorizer, runner)

				resp := api.Post(route, distributedArgs())

				require.Equal(t, tt.status, resp.Code)
				problem := decodeProblem(t, resp.Body.Bytes())
				assert.Equal(t, tt.status, problem.Status)
				assert.Equal(t, tt.detail, problem.Detail)
				assert.NotContains(t, resp.Body.String(), "secret detail", "authorizer error text must not be returned")
				assert.Empty(t, resp.Header().Get("WWW-Authenticate"))
				assert.Zero(t, runner.calls, "runner must not be called")
			})
		}
	}
}

func TestAuthorization_AuthorizedRequest_RunnerObservesScope(t *testing.T) {
	for _, route := range []string{QueryRoute, SSEQueryRoute} {
		t.Run(route, func(t *testing.T) {
			runner := &scopeRecordingRunner{}
			api := setupAuthorizedAPI(t, stubAuthorizer{scope: stubScope{principal: "alice"}}, runner)

			resp := api.Post(route, distributedArgs())

			require.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, []string{"alice"}, runner.principals)
		})
	}
}

func TestAuthorization_NoAuthorizer_RunnerSeesNoScope(t *testing.T) {
	runner := &scopeRecordingRunner{}
	_, api := humatest.New(t)
	RegisterQueryAPI(api, "test", runner, nil)

	resp := api.Post(QueryRoute, distributedArgs())

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, 1, runner.calls)
	assert.Empty(t, runner.principals)
}

func TestAuthorization_ValidationRoute_IsNotAuthorized(t *testing.T) {
	api := setupAuthorizedAPI(t, stubAuthorizer{err: authz.ErrUnauthenticated}, &scopeRecordingRunner{})

	resp := api.Post(ValidationRoute, distributedArgs())

	require.Equal(t, http.StatusNoContent, resp.Code)
}
