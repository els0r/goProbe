package api

import (
	"expvar"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/els0r/goProbe/pkg/api/errors"
	v1 "github.com/els0r/goProbe/pkg/api/v1"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/discovery"
	log "github.com/els0r/log"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/docgen"
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
	logger  log.Logger
	metrics bool

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

func getLoggerHandler(logger log.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
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
func New(port string, manager *capture.Manager, opts ...Option) (*Server, error) {

	s := &Server{
		root:                  "/",
		keys:                  make(map[string]struct{}),
		timeout:               DefaultTimeout * time.Second,
		perMinRateLimit:       DefaultPerMinRateLimit,
		burstLimit:            DefaultBurstLimit,
		concurrentAccessLimit: DefaultConcurrentAccessLimit,
		logger:                log.NewDevNullLogger(), // default is logging to nothing
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
		v1.WithLogger(s.logger),
		// inject standard error handler and attach api logger
		v1.WithErrorHandler(errors.NewStandardHandler(s.logger)),
	}
	if s.discoveryConfigUpdate != nil {
		v1Options = append(v1Options, v1.WithDiscoveryConfigUpdate(s.discoveryConfigUpdate))
	}

	s.apis = append(s.apis,
		v1.New(manager, v1Options...),
	)

	r := chi.NewRouter()

	// setup a good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// only use the logging middleware if a logger was specifically provided
	switch s.logger.(type) {
	case *log.DevNullLogger:
	default:
		r.Use(reqLogger) // prints to stdout at the moment
	}
	r.Use(middleware.Recoverer)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(s.timeout))

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

			s.logger.Debugf("Set up rate limiter middleware: req/min=%d, burst=%d, concurrent req=%d", s.perMinRateLimit, s.burstLimit, s.concurrentAccessLimit)

			// attach API authentication handler
			if len(s.keys) > 0 {
				r.Use(s.AuthenticationHandler(s.keys))
				s.logger.Debugf("API authentication handler registered %d key(s)", len(s.keys))
			} else {
				s.logger.Warn("running API without authentication keys exposes it to any (potentially malicious) third-party")
			}

			// mount all APIs
			for _, api := range s.apis {

				s.logger.Infof("Enabling API %s", api.Version())

				// route base on the API version
				r.Mount("/"+api.Version(), api.Routes())
			}
		})
	})

	// expose metrics if needed
	if s.metrics {
		s.logger.Debugf("Enabling metrics export on http://%s%sdebug/vars", s.location, s.root)

		// specify metrics location
		r.Get("/debug/vars", getMetrics)
	}

	s.router = r

	return s, nil
}

// GenerateAPIDocs generates the API documentation
func (s *Server) GenerateAPIDocs(json, md io.Writer) error {
	var err, jerr, mderr error

	// opportunistically write both documentations
	_, jerr = fmt.Fprintf(json, docgen.JSONRoutesDoc(s.router))
	if jerr != nil {
		err = fmt.Errorf("json docgen: %s", jerr)
	}
	_, mderr = fmt.Fprintf(md, docgen.MarkdownRoutesDoc(s.router, docgen.MarkdownOpts{
		ProjectPath: "github.com/els0r/goProbe",
		Intro:       mdAPIDocIntro,
	}))
	if mderr != nil {
		err = fmt.Errorf("%s; md docgen: %s", err, mderr)
	}
	return err
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
