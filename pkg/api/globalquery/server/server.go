// Package server provides the API server implementation for the global-query service
package server

import (
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/v4/cmd/global-query/pkg/conf"
	gqdistributed "github.com/els0r/goProbe/v4/cmd/global-query/pkg/distributed"
	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/api/server"
	"github.com/els0r/goProbe/v4/pkg/distributed"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/pkg/version"
)

// Server runs a global-query API server
type Server struct {
	hostListResolvers *hosts.ResolverMap
	querier           distributed.Querier

	*server.DefaultServer
}

// New creates a new global-query API server
func New(addr string, resolvers *hosts.ResolverMap, querier distributed.Querier, opts ...server.Option) *Server {
	server := &Server{
		hostListResolvers: resolvers,
		querier:           querier,
		DefaultServer:     server.NewDefault(conf.ServiceName, addr, opts...),
	}

	server.registerRoutes()

	return server
}

func (server *Server) registerRoutes() {
	var (
		middlewares huma.Middlewares
		opts        []gqdistributed.QueryOption
	)

	// an authorizer always comes with runner enforcement: a query never runs unscoped
	// because a middleware was not attached. Authorization runs before the rate limiter
	// so that rejected requests never consume the shared budget. Without an authorizer
	// queries run unscoped; the global-query command warns about that once at startup
	if authorizer, ok := server.Authorizer(); ok {
		middlewares = append(middlewares, api.AuthorizationMiddleware(server.API(), authorizer))
		opts = append(opts, gqdistributed.WithScopeEnforcement())
	}

	maxConcurrentQueries, rateLimiter, enabled := server.QueryRateLimiter()
	if enabled {
		middlewares = append(middlewares, api.RateLimitMiddleware(rateLimiter))
	}

	if maxConcurrentQueries > 0 {
		sem := make(chan struct{}, maxConcurrentQueries)
		opts = append(opts, gqdistributed.WithMaxConcurrent(sem))
	}
	api.RegisterQueryAPI(server.API(),
		fmt.Sprintf("global-query/%s", version.Short()),
		gqdistributed.NewQueryRunner(server.hostListResolvers, server.querier, opts...),
		middlewares,
	)
}
