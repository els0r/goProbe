package api

import (
	log "github.com/els0r/log"
	"github.com/go-chi/chi/middleware"
)

type requestLogger struct {
	l log.Logger
}

// Print outputs messages with priority "info" by default
func (r *requestLogger) Print(v ...interface{}) {
	r.l.Info(v...)
}

var reqLogger = middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: &requestLogger{log.NewTextLogger(log.WithTextPlainOutput())}, NoColor: false})
