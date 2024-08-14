package server

import (
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/api/server"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/goDB/engine"
	"github.com/els0r/goProbe/pkg/version"
)

// Server runs a goprobe API server
type Server struct {

	// goprobe specific variables
	dbPath         string
	captureManager *capture.Manager
	configMonitor  *config.Monitor

	*server.DefaultServer
}

// New creates a new goprobe API server
func New(addr, dbPath string, captureManager *capture.Manager, configMonitor *config.Monitor, opts ...server.Option) *Server {
	server := &Server{
		dbPath:         dbPath,
		captureManager: captureManager,
		configMonitor:  configMonitor,
		DefaultServer:  server.NewDefault(config.ServiceName, addr, opts...),
	}

	server.registerRoutes()

	return server
}

const ifaceKey = "interface"

func (server *Server) registerRoutes() {
	var middlewares huma.Middlewares
	rateLimiter, enabled := server.QueryRateLimiter()
	if enabled {
		middlewares = append(middlewares, api.RateLimitMiddleware(rateLimiter))
	}

	// query
	api.RegisterQueryAPI(server.API(),
		fmt.Sprintf("goProbe/%s", version.Short()),
		engine.NewQueryRunnerWithLiveData(server.dbPath, server.captureManager),
		middlewares,
	)

	// stats
	server.registerStatusAPI()

	// config
	server.registerConfigAPI()
}
