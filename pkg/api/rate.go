package api

import (
	"net/http"

	"github.com/throttled/throttled"
	"github.com/throttled/throttled/store/memstore"
)

// Default rate limits
const (
	DefaultPerMinRateLimit       = 10
	DefaultBurstLimit            = 5
	DefaultConcurrentAccessLimit = 1 // only one client can connect simultaneously
)

// RateLimiter returns a middleware that rate limits access to the API paths
func (s *Server) RateLimiter(perMinLimit, burstLimit int) func(http.Handler) http.Handler {

	// store rate limiter in memory
	store, err := memstore.New(65536)
	if err != nil {
		s.logger.Errorf("failed to create in-memory rate limiter store: %s", err)

		// failure to set up the rate limiter shouldn't block the entire API
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// set maximum bursts
	quota := throttled.RateQuota{MaxRate: throttled.PerMin(perMinLimit), MaxBurst: burstLimit}

	rateLimiter, err := throttled.NewGCRARateLimiter(store, quota)
	if err != nil {
		s.logger.Errorf("failed to create rate limiter: %s", err)

		// failure to set up the rate limiter shouldn't block the entire API
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	rl := throttled.HTTPRateLimiter{
		RateLimiter: rateLimiter,
		// don't do per-path rate limiting
		VaryBy: &throttled.VaryBy{Path: false},
	}
	return rl.RateLimit
}
