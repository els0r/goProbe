package distributed

import (
	"context"
	"testing"

	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockScope is a Scope that keeps the host IDs in allowed
type mockScope struct {
	principal string
	allowed   []hosts.ID
}

func (s *mockScope) Principal() string { return s.principal }

func (s *mockScope) Filter(_ context.Context, hostIDs hosts.Hosts) (hosts.Hosts, error) {
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

func scopedCtx(scope authz.Scope) context.Context {
	return authz.WithScope(context.Background(), scope)
}

func TestScopedQuery_ExplicitHostList_NarrowedBeforeFanOut(t *testing.T) {
	mr := &mockResolver{out: hosts.Hosts{"h1", "h2", "h3"}}
	rm := hosts.NewResolverMap()
	rm.Set("string", mr)
	mq := &mockQuerier{results: []*results.Result{makeResult("h1", "eth0", 10, 1)}}
	qr := NewQueryRunner(rm, mq, WithScopeEnforcement())

	args := baseArgs()
	args.QueryHosts = "h1,h2,h3"

	scope := &mockScope{principal: "alice", allowed: []hosts.ID{"h1", "h3"}}
	res, err := qr.Run(scopedCtx(scope), args)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.ElementsMatch(t, hosts.Hosts{"h1", "h3"}, mq.lastHosts)
}
