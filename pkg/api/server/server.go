package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/els0r/goProbe/pkg/api"
	"github.com/gin-gonic/gin"
)

const (
	maxMultipartMemory = 32 << 20 // 32 MiB
)

type Option func(*DefaultServer)

// DefaultServer is the default API server, allowing middlewares and settings to be
// re-used across binaries serving an API
type DefaultServer struct {
	// api handling
	// TODO: authorize API access
	keys []string

	debug bool

	// telemetry
	profiling bool
	metrics   bool

	addr string

	srv    *http.Server
	router *gin.Engine

	unixSocketFile string
}

// WithDebugMode runs the gin server in debug mode (e.g. not setting the release mode)
func WithDebugMode(enabled bool) Option {
	return func(server *DefaultServer) {
		server.debug = enabled
	}
}

// WithProfiling enables runtime profiling endpoints
func WithProfiling(enabled bool) Option {
	return func(server *DefaultServer) {
		server.profiling = enabled
	}
}

// WithMetrics enables prometheus metrics endpoints
func WithMetrics(enabled bool) Option {
	return func(server *DefaultServer) {
		server.metrics = enabled
	}
}

// NewDefault creates a new API server
func NewDefault(addr string, opts ...Option) *DefaultServer {
	s := &DefaultServer{
		addr: addr,
	}

	router := gin.New()
	router.MaxMultipartMemory = maxMultipartMemory

	router.Use(gin.Recovery())

	// make sure that unix sockets are handled if they are provided
	s.unixSocketFile = api.ExtractUnixSocket(addr)

	s.router = router
	for _, opt := range opts {
		opt(s)
	}

	if !s.debug {
		gin.SetMode(gin.ReleaseMode)
	}

	s.registerMiddlewares()

	return s
}

// Router returns the gin.Engine used by the DefaultServer
func (server *DefaultServer) Router() *gin.Engine {
	return server.router
}

func (server *DefaultServer) registerMiddlewares() {
	server.router.Use(
		api.TraceIDMiddleware(),
		api.RequestLoggingMiddleware(),
	)

	if server.profiling {
		api.RegisterProfiling(server.router)
	}
}

const headerTimeout = 30 * time.Second

// Serve starts the API server after adding additional (optional) routes
func (server *DefaultServer) Serve() error {
	server.srv = &http.Server{
		Handler:           server.router.Handler(),
		ReadHeaderTimeout: headerTimeout,
	}

	// listen on UNIX socket
	if server.unixSocketFile != "" {
		listener, err := net.Listen("unix", server.unixSocketFile)
		if err != nil {
			return err
		}
		return server.srv.Serve(listener)
	}

	// listen on address
	server.srv.Addr = server.addr
	return server.srv.ListenAndServe()
}

// Shutdown shuts down the API server
func (server *DefaultServer) Shutdown(ctx context.Context) error {
	return server.srv.Shutdown(ctx)
}
