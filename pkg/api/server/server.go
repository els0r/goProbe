package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/goDB/info"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/els0r/telemetry/metrics"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"golang.org/x/time/rate"
)

const (

	// RuntimeIDHeaderKey denotes the header name / key that identifies the server runtime ID
	RuntimeIDHeaderKey = "X-GOPROBE-RUNTIME-ID"

	maxMultipartMemory = 32 << 20 // 32 MiB
)

// Option denotes a functional option fo a default server instance
type Option func(*DefaultServer)

// DefaultServer is the default API server, allowing middlewares and settings to be
// re-used across binaries serving an API
type DefaultServer struct {
	// api handling
	// TODO: authorize API access
	keys []string

	debug bool

	// telemetry
	profiling              bool
	metrics                bool
	tracing                bool
	requestDurationBuckets []float64

	serviceName string // serviceName is the name of the program that serves the API, e.g. global-query
	addr        string

	// global rate limiting for queries
	queryRateLimiter *rate.Limiter

	srv    *http.Server
	router *gin.Engine
	api    huma.API

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

// WithMetrics enables prometheus metrics endpoints. The request duration can be provided if they should differ
// from the default duration buckets
func WithMetrics(enabled bool, requestDurationBuckets ...float64) Option {
	return func(server *DefaultServer) {
		server.metrics = enabled
		server.requestDurationBuckets = requestDurationBuckets
	}
}

// WithTracing enables trace propagation
func WithTracing(enabled bool) Option {
	return func(server *DefaultServer) {
		server.tracing = enabled
	}
}

// WithQueryRateLimit enables a global rate limit for query calls
func WithQueryRateLimit(r rate.Limit, b int) Option {
	return func(server *DefaultServer) {
		if r > 0. {
			server.queryRateLimiter = rate.NewLimiter(r, b)
		}
	}
}

// NewDefault creates a new API server
func NewDefault(serviceName, addr string, opts ...Option) *DefaultServer {
	s := &DefaultServer{
		addr: addr,
		// make sure that serviceName conforms to the prometheus naming convention. Exhaustive would be stripping
		// the serviceName off any characters that are not permitted
		serviceName: strings.ToLower(serviceName),
	}

	// Set Gin release / debug mode according to debug flag (must happen _before_ call to gin.New())
	if !s.debug {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.MaxMultipartMemory = maxMultipartMemory

	router.Use(gin.Recovery())
	router.Use(cors.Default())

	// make sure that unix sockets are handled if they are provided
	s.unixSocketFile = api.ExtractUnixSocket(addr)

	s.router = router
	for _, opt := range opts {
		opt(s)
	}

	// get a documented API
	s.api = humagin.New(s.router, huma.DefaultConfig(serviceName, version.Short()))

	// register info routes before any other middleware so they are exempt from logging
	// and/or tracing
	s.registerInfoRoutes()

	s.registerMiddlewares()

	return s
}

// API returns the huma API server which is used to register and document endpoints
func (server *DefaultServer) API() huma.API {
	return server.api
}

// OpenAPI writes the full OpenAPI spec to the writer w
func (server *DefaultServer) OpenAPI(w io.Writer) error {
	b, err := server.api.OpenAPI().DowngradeYAML()
	if err != nil {
		return fmt.Errorf("failed to generate OpenAPI spec: %w", err)
	}
	_, err = w.Write(b)
	return err
}

// QueryRateLimiter returns the global rate limiter, if enabled (if not it return nil and false)
func (server *DefaultServer) QueryRateLimiter() (*rate.Limiter, bool) {
	return server.queryRateLimiter, server.queryRateLimiter != nil
}

func (server *DefaultServer) registerInfoRoutes() {
	huma.Register(server.api, api.GetHealthOperation(), api.GetHealthHandler())
	huma.Register(server.api, api.GetInfoOperation(), api.GetServiceInfoHandler(server.serviceName))
	huma.Register(server.api, api.GetReadyOperation(), api.GetReadyHandler())
}

func (server *DefaultServer) registerMiddlewares() {
	var middlewares []gin.HandlerFunc
	if server.tracing {
		middlewares = append(middlewares,
			otelgin.Middleware(server.serviceName, otelgin.WithFilter(
				// make sure the excluded endpoints don't get traced
				func(req *http.Request) bool {
					for _, path := range []string{
						api.InfoRoute, api.HealthRoute, api.ReadyRoute,
					} {
						// paths prefixed with /- should also be excluded. It's a convention pushed in prometheus projects and quite used in osag
						if req.URL.Path == path || req.URL.Path == "/-"+path {
							return false
						}
					}
					return true
				},
			)),
		)
	}

	middlewares = append(middlewares,
		api.TraceIDMiddleware(),
		api.RequestLoggingMiddleware(),
		api.RecursionDetectorMiddleware(RuntimeIDHeaderKey, info.RuntimeID()),
	)

	server.router.Use(middlewares...)

	if server.metrics {
		buckets := prometheus.DefBuckets
		if len(server.requestDurationBuckets) > 0 {
			buckets = server.requestDurationBuckets
		}
		metrics.NewPrometheus(server.serviceName, "api").
			WithRequestDurationBuckets(buckets).
			Register(server.router)
	}
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
