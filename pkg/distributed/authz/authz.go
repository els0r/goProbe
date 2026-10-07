// Package authz defines the authorization contract for scoped distributed queries.
//
// An Authorizer turns a request into a Scope, or rejects it. A Scope belongs to one
// principal and narrows the list of host IDs a query reaches. The vocabulary (host ID,
// principal, scope, authorizer) is defined in CONTEXT.md, the design in ADR 0003.
//
// The package depends on the standard library and the host ID type only, so that
// out-of-tree authorizers can build against it.
package authz

import (
	"context"
	"crypto/tls"
	"errors"

	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
)

var (
	// ErrUnauthenticated is returned by an Authorizer when the request carries no valid
	// credential. It is answered with 401.
	ErrUnauthenticated = errors.New("unauthenticated")

	// ErrForbidden is returned by an Authorizer when the request carries a valid credential
	// that is not allowed to query at all. It is answered with 403.
	ErrForbidden = errors.New("forbidden")

	// ErrNoAuthorizedHosts denotes that every host ID of a query lies outside its scope.
	// It is answered with 403.
	ErrNoAuthorizedHosts = errors.New("no authorized hosts in query")
)

// Request is the read-only view of an incoming request an Authorizer decides on. It gives
// no access to the body. The web framework's request context satisfies it.
type Request interface {
	// Header returns the first value of the named header, or an empty string
	Header(name string) string

	// EachHeader calls cb for every header name / value pair
	EachHeader(cb func(name, value string))

	// TLS returns the TLS connection state, or nil for a plaintext connection
	TLS() *tls.ConnectionState

	// RemoteAddr returns the remote address of the client
	RemoteAddr() string
}

// Scope is the set of host IDs one principal may query, established once per request
type Scope interface {
	// Principal returns the opaque, non-secret identity of the caller. It is used for
	// audit only
	Principal() string

	// Filter returns the subset of hostIDs the principal may query. It must only narrow:
	// a host ID that was not passed in must not be returned
	Filter(ctx context.Context, hostIDs hosts.Hosts) (hosts.Hosts, error)
}

// Authorizer turns a request into a Scope, or rejects it
type Authorizer interface {
	// Authorize returns the Scope of the request. It returns ErrUnauthenticated when the
	// request carries no valid credential, ErrForbidden when the caller is not allowed to
	// query, and any other error when the decision could not be made
	Authorize(ctx context.Context, req Request) (Scope, error)
}

type scopeContextKey struct{}

// WithScope returns a copy of ctx carrying scope
func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, scope)
}

// ScopeFromContext returns the Scope carried by ctx, if any
func ScopeFromContext(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeContextKey{}).(Scope)
	return scope, ok && scope != nil
}
