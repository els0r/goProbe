package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/telemetry/logging"
)

// Fixed details of the responses the authorization middleware sends. What the authorizer
// reports is logged, never returned to the caller
const (
	DetailUnauthenticated          = "unauthenticated"
	DetailForbidden                = "forbidden"
	DetailAuthorizationUnavailable = "authorization unavailable"
)

// AuthorizationMiddleware turns each request into a scope via authorizer and attaches it to
// the request context. It is meant for the routes that dispatch queries and runs before the
// rate limiter. A request the authorizer rejects is answered with 401 (unauthenticated), 403
// (forbidden) or 503 (any other failure, never fail-open), each with a fixed detail
func AuthorizationMiddleware(a huma.API, authorizer authz.Authorizer) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		scope, err := authorizer.Authorize(ctx.Context(), ctx)
		if err == nil && scope == nil {
			err = errors.New("authorizer returned neither a scope nor an error")
		}
		if err != nil {
			status, detail := authorizationStatus(err)
			logger := logging.FromContext(ctx.Context()).With(
				"authorizer", fmt.Sprintf("%T", authorizer), "status", status,
			)
			logger.Error("authorization rejected request", "error", err)
			if werr := huma.WriteErr(a, ctx, status, detail); werr != nil {
				logger.Error("failed to write authorization response", "error", werr)
			}
			return
		}

		next(huma.WithContext(ctx, authz.WithScope(ctx.Context(), scope)))
	}
}

// authorizationStatus maps an authorizer failure to the HTTP status and fixed detail
func authorizationStatus(err error) (int, string) {
	switch {
	case errors.Is(err, authz.ErrUnauthenticated):
		return http.StatusUnauthorized, DetailUnauthenticated
	case errors.Is(err, authz.ErrForbidden), errors.Is(err, authz.ErrNoAuthorizedHosts):
		return http.StatusForbidden, DetailForbidden
	default:
		return http.StatusServiceUnavailable, DetailAuthorizationUnavailable
	}
}
