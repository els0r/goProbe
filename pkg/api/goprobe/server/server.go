package server

import (
	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/api"
	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/els0r/goProbe/pkg/api/server"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/defaults"
	"github.com/gin-gonic/gin"
)

// Server runs a goprobe API server
type Server struct {

	// goprobe specific variables
	dbPath         string
	captureManager *capture.Manager
	configMonitor  *config.Monitor

	*server.DefaultServer
}

// SetDBPath sets the path to the database directory
func (server *Server) SetDBPath(path string) *Server {
	server.dbPath = path
	return server
}

// New creates a new goprobe API server
func New(addr string, captureManager *capture.Manager, configMonitor *config.Monitor, opts ...server.Option) *Server {
	server := &Server{
		dbPath:         defaults.DBPath,
		captureManager: captureManager,
		configMonitor:  configMonitor,
		DefaultServer:  server.NewDefault(config.ServiceName, addr, opts...),
	}

	server.registerRoutes()

	return server
}

const ifaceKey = "interface"

func (server *Server) registerRoutes() {
	router := server.Router()

	// query
	queryHandlers := gin.HandlersChain{server.postQuery}
	if limiter, hasLimiter := server.QueryRateLimiter(); hasLimiter {
		queryHandlers = append(gin.HandlersChain{api.RateLimitMiddleware(limiter)}, queryHandlers...)
	}
	router.GET(api.QueryRoute, queryHandlers...)  // support for URL-encoded form data GET requests
	router.POST(api.QueryRoute, queryHandlers...) // support for JSON or form-data body POST requests

	// stats
	statsRoutes := router.Group(gpapi.StatusRoute)
	statsRoutes.GET("", server.getStatus)
	statsRoutes.GET("/:"+ifaceKey, server.getStatus)

	// config
	configRoutes := router.Group(gpapi.ConfigRoute)
	configRoutes.GET("", server.getConfig)
	configRoutes.GET("/:"+ifaceKey, server.getConfig)
	configRoutes.PUT("", server.putConfig)
	configRoutes.POST(gpapi.ConfigReloadRoute, server.reloadConfig)
}
