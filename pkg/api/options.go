package api

import (
	"time"

	"github.com/els0r/goProbe/pkg/discovery"
	log "github.com/els0r/log"
)

// Option allows to set optional parameters in the server
type Option func(*Server)

// WithKeys registers API authentication keys with the server
func WithKeys(keys []string) Option {
	return func(s *Server) {
		for _, key := range keys {
			s.keys[key] = struct{}{}
		}
	}
}

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

// WithRateLimits applies custom request rate limits to the API. Setting any of the values to zero deactivates the rate limiting for that particular part.
func WithRateLimits(perMin, burst, concurrentRequests int) Option {
	return func(s *Server) {
		s.perMinRateLimit = perMin
		s.burstLimit = burst
		s.concurrentAccessLimit = concurrentRequests
	}
}

// WithHost sets the host to a specific IP address or hostname
func WithHost(host string) Option {
	return func(s *Server) {
		s.location = host + s.location
	}
}

// WithTimeout allows to override the default request timeout
func WithTimeout(seconds int) Option {
	return func(s *Server) {
		s.timeout = time.Duration(seconds) * time.Second
	}
}

// WithDiscoveryConfigUpdate hands over a probe registration client and enables service discovery
func WithDiscoveryConfigUpdate(update chan *discovery.Config) Option {
	return func(s *Server) {
		s.discoveryConfigUpdate = update
	}
}
