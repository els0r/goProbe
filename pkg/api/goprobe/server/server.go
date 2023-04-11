package server

import (
	"context"
	"net/http"
	"time"

	"github.com/els0r/goProbe/pkg/api"
	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/defaults"
	"github.com/els0r/goProbe/pkg/goprobe/writeout"
	"github.com/gin-gonic/gin"
)

type Option func(*Server)

type Server struct {
	// goprobe specific variables
	dbPath          string
	captureManager  *capture.Manager
	writeoutHandler *writeout.Handler

	// api handling
	// TODO: authorize API access
	keys []string

	addr string

	srv    *http.Server
	router *gin.Engine
}

func WithDBPath(path string) Option {
	return func(server *Server) {
		server.dbPath = path
	}
}

func New(addr string, captureManager *capture.Manager, writeoutHandler *writeout.Handler, opts ...Option) *Server {
	server := &Server{
		addr:            addr,
		dbPath:          defaults.DBPath,
		captureManager:  captureManager,
		writeoutHandler: writeoutHandler,
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

func (server *Server) registerMiddlewares() {
	server.router.Use(
		api.TraceIDMiddleware(),
		api.RequestLoggingMiddleware(),
	)
}

func (server *Server) registerRoutes() {
	server.router.POST(gpapi.QueryRoute, server.postQuery)
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