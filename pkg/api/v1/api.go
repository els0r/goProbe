// Package v1 specifies goProbe API version 1.
//
// Actions (POST):
//
// Path: /
//
//   /_reload:
//    Triggers a reload of the configuration.
//
//    Parameters:
//        None
//
// Statistics (GET):
//
// Path: /stats
//
//   /packets
//    Returns the number of packets received in the last writeout period
//
//    Parameters:
//        * debug: if set to 1, it will print out info for each interface
//        * pretty: if set to 1, it will use status line to print out the statistics. Default format is JSON
//
//   /errors
//    Returns the pcap errors ocurring on each interface
//
//    Parameters:
//        * pretty: if set to 1, it will use status line to print out the statistics. Default format is JSON
//
// Flow State (GET):
//
// Path: /flows
//
//   /all:
//    Prints the active flows for all captured interfaces
//
//    Parameters:
//        * pretty: if set to 1, it will use status line to print out the statistics. Default format is JSON
//
//   /{ifaceName}:
//    Prints the active flows for interface {ifaceName}
//
//    Parameters:
//        * pretty: if set to 1, it will use status line to print out the statistics. Default format is JSON
package v1

import (
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/go-chi/chi"

	log "github.com/els0r/log"
)

// Option can enable/disable API features upon instantiation
type Option func(*API)

// WithLogger adds a logger to the API
func WithLogger(logger log.Logger) Option {
	return func(a *API) {
		a.logger = logger
	}
}

// API holds access to goProbe's internal capture routines
type API struct {
	c      *capture.Manager
	logger log.Logger
}

// New creates a new API
func New(manager *capture.Manager, opts ...Option) *API {
	a := &API{c: manager}

	// apply options
	for _, opt := range opts {
		opt(a)
	}

	if a.logger != nil {
		a.logger.Debugf("Enabling API %s", a.Version())
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
