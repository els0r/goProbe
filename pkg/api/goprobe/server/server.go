package server

import (
	"context"
	"net"
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

	addr           string
	unixSocketFile string

	srv *http.Server

	router *gin.Engine
}

func WithDBPath(path string) Option {
	return func(server *Server) {
		server.dbPath = path
	}
}

// TODO: support for unix sockets

func New(addr string, captureManager *capture.Manager, opts ...Option) *Server {
	server := &Server{
		addr:           addr,
		dbPath:         defaults.DBPath,
		captureManager: captureManager,
	}

	router := gin.New()
	router.MaxMultipartMemory = 32 << 20 // 32 MiB

	router.Use(gin.Recovery())

	// make sure that unix sockets are handled if they are provided
	server.unixSocketFile = api.ExtractUnixSocket(addr)

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

const ifaceKey = "interface"

func (server *Server) registerRoutes() {
	// query
	server.router.POST(gpapi.QueryRoute, server.postQuery)

	// stats
	statsRoutes := server.router.Group(gpapi.StatusRoute)
	statsRoutes.GET("", server.getStatus)
	statsRoutes.GET("/:"+ifaceKey, server.getStatus)

	// config
	configRoutes := server.router.Group(gpapi.ConfigRoute)
	configRoutes.GET("", server.getConfig)
	configRoutes.GET("/:"+ifaceKey, server.getConfig)
	configRoutes.POST("", server.putConfig)
}

const headerTimeout = 30 * time.Second

// Serve starts the API server after adding additional (optional) routes
func (server *Server) Serve() error {
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
func (server *Server) Shutdown(ctx context.Context) error {
	return server.srv.Shutdown(ctx)
}
