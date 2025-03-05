package server

import (
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/v4/cmd/goProbe/config"
	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/api/server"
	"github.com/els0r/goProbe/v4/pkg/capture"
	"github.com/els0r/goProbe/v4/pkg/goDB/engine"
	"github.com/els0r/goProbe/v4/pkg/version"
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
	maxConcurrentQueries, rateLimiter, enabled := server.QueryRateLimiter()
	if enabled {
		middlewares = append(middlewares, api.RateLimitMiddleware(rateLimiter))
	}

	// query
	opts := []engine.RunnerOption{
		engine.WithLiveData(server.captureManager),
	}
	if maxConcurrentQueries > 0 {
		sem := make(chan struct{}, maxConcurrentQueries)
		opts = append(opts, engine.WithMaxConcurrent(sem))
	}
	api.RegisterQueryAPI(server.API(),
		fmt.Sprintf("goProbe/%s", version.Short()),
		engine.NewQueryRunner(server.dbPath, opts...),
		middlewares,
	)

	// stats
	server.registerStatusAPI()

	// config
	server.registerConfigAPI()
}
