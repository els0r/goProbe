package api

import (
	"expvar"
	"fmt"
	"net/http"
	"time"

	"github.com/els0r/goProbe/pkg/api/errors"
	v1 "github.com/els0r/goProbe/pkg/api/v1"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/discovery"
	"github.com/els0r/goProbe/pkg/goprobe/writeout"
	"github.com/els0r/goProbe/pkg/logging"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// API is any type that exposes its URL paths via chi.Routes
type API interface {
	// Routes returns the accessible functions of the API
	Routes() *chi.Mux

	// Version returns the API version
	Version() string
}

// Server is the entry path for goProbe's http server and holds all API versions and routes
type Server struct {
	// base config
	location string // what the server binds to
	root     string // document root

	// path setup
	router chi.Router
	apis   []API

	// authentication
	keys map[string]struct{}

	// info
	logRequests bool
	metrics     bool

	// throttling parameters
	perMinRateLimit       int
	burstLimit            int
	concurrentAccessLimit int

	// timeouts
	timeout time.Duration

	// discovery config update
	discoveryConfigUpdate chan *discovery.Config
}

// Keys allows for quick key validation
type Keys map[string]struct{}

// unique keys for context setting
type key int

const (
	apiKeyCtxKey key = iota
)

// errors
const (
	errContextCanceled = "Context was canceled."
)

const (
	// DefaultTimeout stores the default request timeout in seconds
	DefaultTimeout = 30
)

func getLoggerHandler() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			logger := logging.FromContext(r.Context())
			logger.Debugf("%s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func getMetrics(w http.ResponseWriter, r *http.Request) {
	expvar.Handler().ServeHTTP(w, r)
}

// New creates a base router for goProbe's APIs and provides the metrics export out of the box
func New(port string, manager *capture.Manager, handler *writeout.Handler, opts ...Option) (*Server, error) {

	s := &Server{
		root:                  "/",
		keys:                  make(map[string]struct{}),
		timeout:               DefaultTimeout * time.Second,
		perMinRateLimit:       DefaultPerMinRateLimit,
		burstLimit:            DefaultBurstLimit,
		concurrentAccessLimit: DefaultConcurrentAccessLimit,
	}

	if port == "" {
		return nil, fmt.Errorf("no port provided")
	}
	s.location = ":" + port
	location := s.location

	// apply options
	for _, opt := range opts {
		opt(s)
	}

	// explicitly set host if not set by options
	if s.location == location {
		s.location = "localhost" + s.location
	}

	// initialize currently supported APIs
	v1Options := []v1.Option{
		// inject standard error handler
		v1.WithErrorHandler(errors.NewStandardHandler()),
	}
	if s.discoveryConfigUpdate != nil {
		v1Options = append(v1Options, v1.WithDiscoveryConfigUpdate(s.discoveryConfigUpdate))
	}

	s.apis = append(s.apis,
		v1.New(manager, handler, v1Options...),
	)

	r := chi.NewRouter()

	// setup a good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// only use the logging middleware if a logger was specifically provided
	if s.logRequests {
		r.Use(reqLogger) // prints to stdout at the moment
	}
	r.Use(middleware.Recoverer)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(s.timeout))

	logger := logging.Logger()

	// set up request routing
	r.Route(s.root, func(r chi.Router) {
		r.Route("/api", func(r chi.Router) {

			// attach rate-limiting middlewares
			if !(s.perMinRateLimit == 0 || s.burstLimit == 0) {
				r.Use(s.RateLimiter(s.perMinRateLimit, s.burstLimit))
			}
			if s.concurrentAccessLimit != 0 {
				r.Use(middleware.Throttle(s.concurrentAccessLimit))
			}

			logger.With(
				"req/min", s.perMinRateLimit,
				"burst", s.burstLimit,
				"max_concurrent", s.concurrentAccessLimit,
			).Debugf("set up rate limiter middleware")

			// attach API authentication handler
			if len(s.keys) > 0 {
				r.Use(s.AuthenticationHandler(s.keys))
				logger.Debugf("API authentication handler registered %d key(s)", len(s.keys))
			} else {
				logger.Warn("running API without authentication keys exposes it to any (potentially malicious) third-party")
			}

			// mount all APIs
			for _, api := range s.apis {
				logger.Infof("enabling API %s", api.Version())

				// route base on the API version
				r.Mount("/"+api.Version(), api.Routes())
			}
		})
	})

	// expose metrics if needed
	if s.metrics {
		logger.Debugf("enabling metrics export on http://%s%sdebug/vars", s.location, s.root)

		// specify metrics location
		r.Get("/debug/vars", getMetrics)
	}

	s.router = r

	return s, nil
}

// Run launches the server to listen on a specified locations (e.g. 127.0.0.1:6060)
func (s *Server) Run() {
	go http.ListenAndServe(s.location, s.router)
}

// ReturnStatus is a wrapper around the default http.Error method
func ReturnStatus(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}

// SupportedAPIs returns a list of all APIs that are registered
func (s *Server) SupportedAPIs() []string {
	var sup []string
	for _, api := range s.apis {
		sup = append(sup, api.Version())
	}
	return sup
}
