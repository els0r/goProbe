package distributed

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/goProbe/v4/plugins/resolver/stringresolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockScope is a Scope that keeps the host IDs in allowed, unless filter overrides it
type mockScope struct {
	principal string
	allowed   []hosts.ID
	filter    func(hosts.Hosts) (hosts.Hosts, error)
}

func (s *mockScope) Principal() string { return s.principal }

func (s *mockScope) Filter(_ context.Context, hostIDs hosts.Hosts) (hosts.Hosts, error) {
	if s.filter != nil {
		return s.filter(hostIDs)
	}
	var out hosts.Hosts
	for _, id := range hostIDs {
		for _, allowed := range s.allowed {
			if id == allowed {
				out = append(out, id)
				break
			}
		}
	}
	return out, nil
}

// perHostQuerier answers one result per host ID it is given and an error result for every
// host ID in missing, mirroring a sensor that does not exist
type perHostQuerier struct {
	lastHosts hosts.Hosts
	missing   []hosts.ID
}

func (q *perHostQuerier) Query(_ context.Context, queryHosts hosts.Hosts, _ *query.Args) (<-chan *results.Result, <-chan struct{}) {
	q.lastHosts = queryHosts
	rc := make(chan *results.Result, len(queryHosts))
	for _, id := range queryHosts {
		if slices.Contains(q.missing, id) {
			r := results.New()
			r.Hostname = id
			r.SetErr(errors.New("host not found"))
			rc <- r
			continue
		}
		r := makeResult(id, "eth0", 10, 1)
		r.HostsStatuses[id] = results.Status{Code: types.StatusOK}
		rc <- r
	}
	close(rc)
	kc := make(chan struct{})
	close(kc)
	return rc, kc
}

func scopedCtx(scope authz.Scope) context.Context {
	return authz.WithScope(context.Background(), scope)
}

type runFunc func(qr *QueryRunner, ctx context.Context, args *query.Args) (*results.Result, error)

// runModes runs a query through the plain and the streaming entry point of the runner
var runModes = []struct {
	name string
	run  runFunc
}{
	{
		name: "plain",
		run: func(qr *QueryRunner, ctx context.Context, args *query.Args) (*results.Result, error) {
			return qr.Run(ctx, args)
		},
	},
	{
		name: "streaming",
		run: func(qr *QueryRunner, ctx context.Context, args *query.Args) (*results.Result, error) {
			sender := &captureSender{}
			return qr.RunStreaming(ctx, args, sse.Sender(sender.send))
		},
	},
}

func TestScopedQuery_ExplicitHostList_NarrowedBeforeFanOut(t *testing.T) {
	for _, mode := range runModes {
		t.Run(mode.name, func(t *testing.T) {
			mr := &mockResolver{out: hosts.Hosts{"h1", "h2", "h3"}}
			rm := hosts.NewResolverMap()
			rm.Set("string", mr)
			mq := &mockQuerier{results: []*results.Result{makeResult("h1", "eth0", 10, 1)}}
			qr := NewQueryRunner(rm, mq, WithScopeEnforcement())

			args := baseArgs()
			args.QueryHosts = "h1,h2,h3"

			scope := &mockScope{principal: "alice", allowed: []hosts.ID{"h1", "h3"}}
			res, err := mode.run(qr, scopedCtx(scope), args)
			require.NoError(t, err)
			require.NotNil(t, res)

			assert.ElementsMatch(t, hosts.Hosts{"h1", "h3"}, mq.lastHosts)
		})
	}
}

func TestScopedQuery_AllHostsSelector_EnumeratedThenNarrowed(t *testing.T) {
	rm := hosts.NewResolverMap()
	rm.Set("string", &mockResolver{out: hosts.Hosts{"should-not-be-used"}})
	mq := &mockQuerier{anyHosts: hosts.Hosts{"any1", "any2", "any3"}}
	qr := NewQueryRunner(rm, mq, WithScopeEnforcement())

	args := baseArgs()
	args.QueryHosts = types.AnySelector

	scope := &mockScope{principal: "alice", allowed: []hosts.ID{"any2"}}
	_, err := qr.Run(scopedCtx(scope), args)
	require.NoError(t, err)

	assert.Equal(t, hosts.Hosts{"any2"}, mq.lastHosts)
}

func TestScopedQuery_EveryResolverType_Narrowed(t *testing.T) {
	tests := []struct {
		name         string
		resolverType string
		resolver     hosts.Resolver
		queryHosts   string
	}{
		{
			name:         "string resolver",
			resolverType: stringresolver.Type,
			resolver:     stringresolver.NewResolver(true),
			queryHosts:   "h1,h2,h3",
		},
		{
			name:         "static resolver",
			resolverType: "static",
			resolver:     &mockResolver{out: hosts.Hosts{"h1", "h2", "h3"}},
			queryHosts:   "ignored-by-static-resolver",
		},
		{
			name:         "default resolver",
			resolverType: "",
			resolver:     &mockResolver{out: hosts.Hosts{"h1", "h2", "h3"}},
			queryHosts:   "h1,h2,h3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := hosts.NewResolverMap()
			registeredAs := tt.resolverType
			if registeredAs == "" {
				registeredAs = "string"
			}
			rm.Set(registeredAs, tt.resolver)
			mq := &mockQuerier{}
			qr := NewQueryRunner(rm, mq, WithScopeEnforcement())

			args := baseArgs()
			args.QueryHosts = tt.queryHosts
			args.QueryHostsResolverType = tt.resolverType

			scope := &mockScope{principal: "alice", allowed: []hosts.ID{"h2", "h3"}}
			_, err := qr.Run(scopedCtx(scope), args)
			require.NoError(t, err)

			assert.ElementsMatch(t, hosts.Hosts{"h2", "h3"}, mq.lastHosts)
		})
	}
}

// requireStatus asserts that err carries the HTTP status and detail the API will render
func requireStatus(t *testing.T, err error, status int, detail string) {
	t.Helper()
	require.Error(t, err)
	var se huma.StatusError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, status, se.GetStatus())
	assert.Equal(t, detail, se.Error())
}

func TestScopedQuery_NoHostInScope_Forbidden_NothingDispatched(t *testing.T) {
	for _, mode := range runModes {
		t.Run(mode.name, func(t *testing.T) {
			rm := hosts.NewResolverMap()
			rm.Set("string", &mockResolver{out: hosts.Hosts{"h1", "h2"}})
			mq := &mockQuerier{}
			qr := NewQueryRunner(rm, mq, WithScopeEnforcement())

			args := baseArgs()
			args.QueryHosts = "h1,h2"

			scope := &mockScope{principal: "alice", allowed: []hosts.ID{"other"}}
			res, err := mode.run(qr, scopedCtx(scope), args)
			require.Nil(t, res)
			requireStatus(t, err, http.StatusForbidden, "no authorized hosts in query")
			assert.ErrorIs(t, err, authz.ErrNoAuthorizedHosts)

			assert.Nil(t, mq.lastArgs, "querier must not be called")
		})
	}
}

func TestScopedQuery_EnforcementWithoutScope_Fails_NothingDispatched(t *testing.T) {
	for _, mode := range runModes {
		t.Run(mode.name, func(t *testing.T) {
			rm := hosts.NewResolverMap()
			rm.Set("string", &mockResolver{out: hosts.Hosts{"h1", "h2"}})
			mq := &mockQuerier{}
			qr := NewQueryRunner(rm, mq, WithScopeEnforcement())

			args := baseArgs()
			args.QueryHosts = "h1,h2"

			res, err := mode.run(qr, context.Background(), args)
			require.Nil(t, res)
			requireStatus(t, err, http.StatusServiceUnavailable, "authorization unavailable")

			assert.Nil(t, mq.lastArgs, "querier must not be called")
		})
	}
}

func TestScopedQuery_ScopeFilterFailure_Unavailable_NothingDispatched(t *testing.T) {
	tests := []struct {
		name   string
		filter func(hosts.Hosts) (hosts.Hosts, error)
	}{
		{
			name: "scope returns a host ID it was not given",
			filter: func(given hosts.Hosts) (hosts.Hosts, error) {
				return append(given[:1:1], "widened"), nil
			},
		},
		{
			name: "scope filter fails",
			filter: func(hosts.Hosts) (hosts.Hosts, error) {
				return nil, errors.New("provider unreachable: secret detail")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := hosts.NewResolverMap()
			rm.Set("string", &mockResolver{out: hosts.Hosts{"h1", "h2"}})
			mq := &mockQuerier{}
			qr := NewQueryRunner(rm, mq, WithScopeEnforcement())

			args := baseArgs()
			args.QueryHosts = "h1,h2"

			scope := &mockScope{principal: "alice", filter: tt.filter}
			res, err := qr.Run(scopedCtx(scope), args)
			require.Nil(t, res)
			requireStatus(t, err, http.StatusServiceUnavailable, "authorization unavailable")
			assert.NotContains(t, err.Error(), "secret detail")

			assert.Nil(t, mq.lastArgs, "querier must not be called")
		})
	}
}

func TestScopedQuery_ResultCarriesOnlyHostsInScope(t *testing.T) {
	for _, mode := range runModes {
		t.Run(mode.name, func(t *testing.T) {
			rm := hosts.NewResolverMap()
			rm.Set("string", &mockResolver{out: hosts.Hosts{"h1", "dropped", "unknown"}})
			pq := &perHostQuerier{missing: []hosts.ID{"unknown", "dropped"}}
			qr := NewQueryRunner(rm, pq, WithScopeEnforcement())

			args := baseArgs()
			args.QueryHosts = "h1,dropped,unknown"

			scope := &mockScope{principal: "alice", allowed: []hosts.ID{"h1", "unknown"}}
			res, err := mode.run(qr, scopedCtx(scope), args)
			require.NoError(t, err)
			require.NotNil(t, res)

			assert.ElementsMatch(t, hosts.Hosts{"h1", "unknown"}, pq.lastHosts)
			assert.NotContains(t, res.HostsStatuses, "dropped")
			assert.Equal(t, types.StatusOK, res.HostsStatuses["h1"].Code)
			assert.Equal(t, types.StatusError, res.HostsStatuses["unknown"].Code, "unknown host ID inside the scope is still reported")
			for _, row := range res.Rows {
				assert.NotEqual(t, "dropped", row.Labels.Hostname)
			}
		})
	}
}

func TestUnscopedQuery_RunnerWithoutEnforcement_IgnoresScope(t *testing.T) {
	rm := hosts.NewResolverMap()
	rm.Set("string", &mockResolver{out: hosts.Hosts{"h1", "h2", "h3"}})
	mq := &mockQuerier{}
	qr := NewQueryRunner(rm, mq)

	args := baseArgs()
	args.QueryHosts = "h1,h2,h3"

	scope := &mockScope{principal: "alice", allowed: []hosts.ID{"h1"}}
	_, err := qr.Run(scopedCtx(scope), args)
	require.NoError(t, err)

	assert.ElementsMatch(t, hosts.Hosts{"h1", "h2", "h3"}, mq.lastHosts)
}
