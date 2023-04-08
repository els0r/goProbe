package api

import (
	"fmt"

	"github.com/els0r/goProbe/pkg/logging"
	"github.com/go-chi/chi/v5/middleware"
)

type requestLogger struct{}

// Print outputs messages with priority "info" by default
func (r *requestLogger) Print(v ...interface{}) {
	logger := logging.Logger()
	logger.Info(fmt.Sprint(v...))
}

var reqLogger = middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: &requestLogger{}, NoColor: false})
