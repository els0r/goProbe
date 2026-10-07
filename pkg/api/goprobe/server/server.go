package server

import (
	"errors"
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

// errAuthorizerUnsupported is raised when the sensor API server is handed an authorizer:
// only global-query scopes queries (ADR 0003), a sensor would silently ignore it
var errAuthorizerUnsupported = errors.New("goProbe API server does not support an authorizer: only global-query scopes queries")

// New creates a new goprobe API server. It panics when an authorizer is configured, since
// the sensor does not scope queries and must not pretend to
func New(addr, dbPath string, captureManager *capture.Manager, configMonitor *config.Monitor, opts ...server.Option) *Server {
	server := &Server{
		dbPath:         dbPath,
		captureManager: captureManager,
		configMonitor:  configMonitor,
		DefaultServer:  server.NewDefault(config.ServiceName, addr, opts...),
	}
	if _, ok := server.Authorizer(); ok {
		panic(errAuthorizerUnsupported)
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
