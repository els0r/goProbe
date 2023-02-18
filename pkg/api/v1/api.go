// Package v1 specifies goProbe API version 1.
package v1

import (
	"net/http"

	"github.com/els0r/goProbe/pkg/api/errors"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/discovery"
	"github.com/go-chi/chi/v5"
)

// Option can enable/disable API features upon instantiation
type Option func(*API)

// WithErrorHandler sets the error handling behavior
func WithErrorHandler(handler errors.Handler) Option {
	return func(a *API) {
		a.errorHandler = handler
	}
}

// WithDiscoveryConfigUpdate hands over a probe registration client and enables service discovery
func WithDiscoveryConfigUpdate(update chan *discovery.Config) Option {
	return func(a *API) {
		a.discoveryConfigUpdate = update
	}
}

// API holds access to goProbe's internal capture routines
type API struct {
	c                     *capture.Manager
	discoveryConfigUpdate chan *discovery.Config
	errorHandler          errors.Handler
}

// New creates a new API
func New(manager *capture.Manager, opts ...Option) *API {
	a := &API{
		c:            manager,
		errorHandler: errors.NewStandardHandler(), // a bare error handler is necessary
	}

	// apply options
	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Version prints the API's version in format "v{versionNumber}"
func (a *API) Version() string {
	return "v1"
}

// Routes sets up the API specific routes. This is the main method to actually connect the API to a server
func (a *API) Routes() *chi.Mux {
	r := chi.NewRouter()

	r.Route("/", func(r chi.Router) {
		// action routes
		a.postRequestRoutes(r)

		// getter routes
		a.getRequestRoutes(r)
	})

	return r
}

func printPretty(r *http.Request) bool {
	return r.FormValue("pretty") == "1"
}
