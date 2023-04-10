package api

import (
	"time"

	"github.com/els0r/goProbe/pkg/logging"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
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
			ctx = logging.NewContext(ctx, slog.String(traceIDKey, sc.TraceID().String()))
		}

		// pass the context through the request context
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func RequestLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := logging.WithContext(c.Request.Context())

		// call next handlers
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		statusCode := c.Request.Response.StatusCode
		logger = logger.With("req", slog.GroupValue(
			slog.String("method", c.Request.Method),
			slog.String("url", c.Request.RequestURI),
			slog.Duration("duration", duration),
		)).With("resp", slog.GroupValue(
			slog.Int("status_code", statusCode),
		))

		if 200 <= statusCode && statusCode < 300 {
			logger.Info("successful request")
		} else {
			logger.Error("failed request")
		}
	}
}
