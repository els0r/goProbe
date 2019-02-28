// Package api provides the methods for goProbe's control server
//
// Base path: /
//
// Program metrics for instrumentation (GET)
// Path: /debug/vars
//    Returns all metrics exposed via the "expvar" library
//
// Path: api/v1/
// Access to API version 1 functions

package api

import (
	"expvar"
	"fmt"
	"net/http"
	"time"

	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/capture/api/v1"
	log "github.com/els0r/log"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

// API is any type that exposes its URL paths via chi.Routes
type API interface {
	// Routes returns the accessible functions of the API
	Routes() *chi.Mux

	// Version returns the API version
	Version() string
}

type Server struct {
	location string // what the server binds to
	root     string // document root

	router chi.Router
	apis   []API

	logger  log.Logger
	metrics bool
}

// Option allows to set optional parameters in the server
type Option func(*Server)

// WithLogger provides the api with access to the program level logger. It is recommended to use this option
func WithLogger(l log.Logger) Option {
	return func(s *Server) {
		s.logger = l
	}
}

// WithMetricsExport switches on metrics export
func WithMetricsExport() Option {
	return func(s *Server) {
		s.metrics = true
	}
}

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
func New(host, port string, manager *capture.Manager, opts ...Option) (*Server, error) {

	s := &Server{root: "/"}

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		return nil, fmt.Errorf("no port provided")
	}
	s.location = host + ":" + port

	// apply options
	for _, opt := range opts {
		opt(s)
	}

	// initialize currently supported APIs
	s.apis = append(s.apis, v1.New(manager, v1.WithLogger(s.logger)))

	r := chi.NewRouter()

	// setup a good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// only use the logging middleware if a logger was specifically provided
	if s.logger != nil {
		r.Use(reqLogger) // prints to stdout at the moment
	}
	r.Use(middleware.Recoverer)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(30 * time.Second))

	// set up request routing
	r.Route(s.root, func(r chi.Router) {

		// mount all APIs
		for _, api := range s.apis {
			// route base on the API version
			r.Mount("/api/"+api.Version(), api.Routes())
		}
	})

	// expose metrics if needed
	if s.metrics {
		if s.logger != nil {
			s.logger.Debugf("Enabling metrics export on http://%s:%s%sdebug/vars", host, port, s.root)
		}

		// specify metrics location
		r.Get("/debug/vars", getMetrics)
	}

	s.router = r
	return s, nil
}

func (s *Server) Run() {
	go http.ListenAndServe(s.location, s.router)
}
