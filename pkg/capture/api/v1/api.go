// Package v1 specifies goProbe API version 1
//
// Actions (POST):
// Path: /actions
//  - /_reload:
//      Triggers a reload of the configuration.
//      Parameters:
//          None
//
// Statistics (GET):
// Path: /stats
// Paramters:
//    * pretty: if set to 1, it will use status line to print out the statistics. Default format is JSON
// - /packets
//    Returns the number of packets received in the last writeout period
//    Parameters:
//        * debug
// - /errors
//    Returns the pcap errors ocurring on each interface
//
// Flow State (GET):
// Path: /flows
// - /all:
//    Prints the active flows for all captured interfaces
//
//- /{ifaceName}:
//    Prints the active flows for interface {ifaceName}

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

	// action routes
	r.Mount("/actions", a.postRequestRoutes())

	// getter routes
	r.Mount("/", a.getRequestRoutes())

	return r
}
