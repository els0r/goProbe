package distributed

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/telemetry/logging"
)

// detailAuthorizationUnavailable is the fixed detail of every 503 the runner answers when a
// scoped query cannot be authorized. It never carries the underlying cause
const detailAuthorizationUnavailable = "authorization unavailable"

var (
	errScopeMissing      = errors.New("scope enforcement is on but the query carries no scope")
	errScopeNotNarrowing = errors.New("scope returned host IDs it was not given")
)

// WithScopeEnforcement makes the runner require a scope on every query and narrow the
// resolved host list with it before fan-out. Without it, the runner ignores scopes
func WithScopeEnforcement() QueryOption {
	return func(qr *QueryRunner) {
		qr.enforceScope = true
	}
}

// scopeError is the outcome of a scoped query that cannot proceed. The API renders its
// status and fixed detail; the cause is for errors.Is only and never reaches the caller
type scopeError struct {
	*query.DetailError
	cause error
}

func (e *scopeError) Unwrap() error { return e.cause }

func newScopeError(status int, detail string, cause error) error {
	return &scopeError{
		DetailError: query.NewDetailError(status, errors.New(detail)),
		cause:       cause,
	}
}

// applyScope narrows hostList with the scope carried by ctx. It is the single place a
// scope is applied: after the host list is resolved and before any sensor is contacted
func (q *QueryRunner) applyScope(ctx context.Context, hostList hosts.Hosts) (hosts.Hosts, error) {
	if !q.enforceScope {
		return hostList, nil
	}

	logger := logging.FromContext(ctx)

	scope, ok := authz.ScopeFromContext(ctx)
	if !ok {
		logger.Error("scope enforcement is on but the query carries no scope")
		return nil, newScopeError(http.StatusServiceUnavailable, detailAuthorizationUnavailable, errScopeMissing)
	}

	logger = logger.With("principal", scope.Principal(), "scope_type", fmt.Sprintf("%T", scope))

	scoped, err := scope.Filter(ctx, hostList)
	if err != nil {
		logger.Error("scope filter failed", "error", err)
		return nil, newScopeError(http.StatusServiceUnavailable, detailAuthorizationUnavailable, err)
	}
	if widened := hostsNotIn(scoped, hostList); len(widened) > 0 {
		logger.Error("scope returned host IDs it was not given", "host_ids", widened)
		return nil, newScopeError(http.StatusServiceUnavailable, detailAuthorizationUnavailable, errScopeNotNarrowing)
	}
	if len(scoped) == 0 {
		return nil, newScopeError(http.StatusForbidden, authz.ErrNoAuthorizedHosts.Error(), authz.ErrNoAuthorizedHosts)
	}

	return scoped, nil
}

// hostsNotIn returns the host IDs of candidates that are not part of reference
func hostsNotIn(candidates, reference hosts.Hosts) (missing hosts.Hosts) {
	known := make(map[hosts.ID]struct{}, len(reference))
	for _, id := range reference {
		known[id] = struct{}{}
	}
	for _, id := range candidates {
		if _, ok := known[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}
