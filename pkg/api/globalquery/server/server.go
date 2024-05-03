package server

import (
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/cmd/global-query/pkg/distributed"
	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/api/server"
	"github.com/els0r/goProbe/pkg/version"
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

	rateLimiter, enabled := server.QueryRateLimiter()
	if enabled {
		middlewares = append(middlewares, api.RateLimitMiddleware(rateLimiter))
	}

	api.RegisterQueryAPI(server.API(),
		fmt.Sprintf("global-query/%s", version.Short()),
		distributed.NewQueryRunner(server.hostListResolver, server.querier),
		middlewares,
	)
}
