package distributed

import (
	"context"

	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
)

// WithScopeEnforcement makes the runner require a scope on every query and narrow the
// resolved host list with it before fan-out. Without it, the runner ignores scopes
func WithScopeEnforcement() QueryOption {
	return func(qr *QueryRunner) {
		qr.enforceScope = true
	}
}

// applyScope narrows hostList with the scope carried by ctx. It is the single place a
// scope is applied: after the host list is resolved and before any sensor is contacted
func (q *QueryRunner) applyScope(ctx context.Context, hostList hosts.Hosts) (hosts.Hosts, error) {
	if !q.enforceScope {
		return hostList, nil
	}

	scope, _ := authz.ScopeFromContext(ctx)
	return scope.Filter(ctx, hostList)
}
