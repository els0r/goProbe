package api

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/els0r/goProbe/pkg/logging"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

const traceIDKey = "traceID"

// TraceIDMiddleware injects a context into a request managed by [go-gin](https://github.com/gin-gonic/gin)
// from which logger/traces can be derived
func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// extract the trace ID from the context if it is present
		sc := trace.SpanContextFromContext(ctx)
		if sc.HasTraceID() {
			ctx = logging.WithFields(ctx, slog.String(traceIDKey, sc.TraceID().String()))
		}

		// pass the context through the request context
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

const requestMsg = "handled request"

func RequestLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := logging.FromContext(c.Request.Context())

		// call next handlers
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		statusCode := c.Writer.Status()
		size := c.Writer.Size()
		// size is set to -1 if there no data written
		if size < 0 {
			size = 0
		}
		logger = logger.With("req", slog.GroupValue(
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.RequestURI),
			slog.String("user-agent", c.Request.UserAgent()),
			slog.Duration("duration", duration),
		)).With("resp", slog.GroupValue(
			slog.Int("status_code", statusCode),
			slog.Int("size", size),
		))

		switch {
		case 200 <= statusCode && statusCode < 300:
			logger.Info(requestMsg)
		case 300 <= statusCode && statusCode < 400:
			logger.Warn(requestMsg)
		case 400 <= statusCode:
			logger.Error(requestMsg)
		}
	}
}

// RateLimitMiddleware creates a global rate limit for all requests, using a maximum of
// r requests per second and a maximum burst rate of b tokens
func RateLimitMiddleware(limiter *rate.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}

// RecursionDetectorMiddleware provides a means to avoid having a distributed querier query itself
// into oblivion
func RecursionDetectorMiddleware(headerKey, match string) gin.HandlerFunc {
	ErrRecursionDetected := errors.New("API query recursion detected, cross-check host configuration")
	return func(c *gin.Context) {
		if c.Request.Header.Get(headerKey) == match {
			logging.FromContext(c.Request.Context()).Error(c.AbortWithError(http.StatusBadRequest, ErrRecursionDetected))
			return
		}
		c.Next()
	}
}

// RegisterProfiling registers the profiling middleware
func RegisterProfiling(router *gin.Engine) {
	pprof.Register(router)
}
