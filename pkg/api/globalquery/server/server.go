package server

import (
	"context"
	"net/http"
	"time"

	"github.com/els0r/goProbe/cmd/global-query/pkg/distributed"
	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/pkg/api"
	gqapi "github.com/els0r/goProbe/pkg/api/globalquery"
	"github.com/gin-gonic/gin"
)

type Option func(*Server)

type Server struct {
	hostListResolver hosts.Resolver
	querier          distributed.Querier

	// TODO: authorize API access
	keys []string

	debug bool

	addr string

	srv    *http.Server
	router *gin.Engine
}

// WithDebugMode runs the gin server in debug mode (e.g. not setting the release mode)
func WithDebugMode(b bool) Option {
	return func(server *Server) {
		server.debug = b
	}
}

func NewServer(addr string, resolver hosts.Resolver, querier distributed.Querier, opts ...Option) *Server {
	server := &Server{
		addr:             addr,
		hostListResolver: resolver,
		querier:          querier,
	}

	router := gin.New()
	router.MaxMultipartMemory = 32 << 20 // 32 MiB

	router.Use(gin.Recovery())

	server.router = router
	for _, opt := range opts {
		opt(server)
	}

	if !server.debug {
		gin.SetMode(gin.ReleaseMode)
	}

	server.registerMiddlewares()
	server.registerRoutes()

	return server
}

func (server *Server) registerMiddlewares() {
	server.router.Use(
		api.TraceIDMiddleware(),
		api.RequestLoggingMiddleware(),
	)
}
func (server *Server) registerRoutes() {
	RegisterQueryHandler(server.router, gqapi.QueryRoute, server.hostListResolver, server.querier)
}

const headerTimeout = 30 * time.Second

// Serve starts the API server after adding additional (optional) routes
func (server *Server) Serve() error {
	server.srv = &http.Server{
		Addr:              server.addr,
		Handler:           server.router,
		ReadHeaderTimeout: headerTimeout,
	}
	return server.srv.ListenAndServe()
}

// Shutdown shuts down the API server
func (server *Server) Shutdown(ctx context.Context) error {
	return server.srv.Shutdown(ctx)
}
