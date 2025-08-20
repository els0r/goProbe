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
	hostListResolver hosts.Resolver
	querier          distributed.Querier

	*server.DefaultServer
}

// New creates a new global-query API server
func New(addr string, resolver hosts.Resolver, querier distributed.Querier, opts ...server.Option) *Server {
	server := &Server{
		hostListResolver: resolver,
		querier:          querier,
		DefaultServer:    server.NewDefault(conf.ServiceName, addr, opts...),
	}

	server.registerRoutes()

	return server
}

func (server *Server) registerRoutes() {
	var middlewares huma.Middlewares

	maxConcurrentQueries, rateLimiter, enabled := server.QueryRateLimiter()
	if enabled {
		middlewares = append(middlewares, api.RateLimitMiddleware(rateLimiter))
	}

	opts := []gqdistributed.QueryOption{}
	if maxConcurrentQueries > 0 {
		sem := make(chan struct{}, maxConcurrentQueries)
		opts = append(opts, gqdistributed.WithMaxConcurrent(sem))
	}
	api.RegisterQueryAPI(server.API(),
		fmt.Sprintf("global-query/%s", version.Short()),
		gqdistributed.NewQueryRunner(server.hostListResolver, server.querier, opts...),
		middlewares,
	)
}
