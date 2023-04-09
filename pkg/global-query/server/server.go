package server

import (
	"context"
	"net/http"
	"time"

	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/gin-gonic/gin"
)

const (
	queryRoute = "/_query"
)

type Option func(*Server)

type Server struct {
	hostListResolver hosts.Resolver
	querier          hosts.Querier

	// TODO: authorize API access
	keys []string

	addr string

	srv    *http.Server
	router *gin.Engine
}

func NewServer(addr string, resolver hosts.Resolver, querier hosts.Querier, opts ...Option) *Server {
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

	server.registerMiddlewares()
	server.registerRoutes()

	return server
}

func (server *Server) registerMiddlewares() {}
func (server *Server) registerRoutes() {
	server.router.POST(queryRoute, server.postQuery)
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
