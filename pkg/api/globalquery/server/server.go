package server

import (
	"github.com/els0r/goProbe/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/cmd/global-query/pkg/distributed"
	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/api/server"
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
	RegisterQueryHandler(server.Router(), api.QueryRoute, server.hostListResolver, server.querier)
	registerQueryValidationRoutes(server.API())
}
