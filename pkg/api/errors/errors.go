package errors

import (
	"fmt"
	"net/http"

	log "github.com/els0r/log"
)

// Handler can be used to determine how an API should deal with providing details
// on errors
type Handler interface {
	// Handle governs how an error is returned to the caller
	Handle(w http.ResponseWriter, statusCode int, err error, msg string)
}

// StandardHandler returns and http.Error and has an internal logger for logging
// errors. It's there to ensure that it is not only apparent that an error occurred
// but also what kind
type StandardHandler struct {
	logger log.Logger
}

// NewStandardHandler returns a new new standard handler.
func NewStandardHandler(l log.Logger) *StandardHandler {
	return &StandardHandler{logger: l}
}

// Handle is a convenience method to return a standard formatted http Error and
// write a log line if a logger is provided
func (s *StandardHandler) Handle(w http.ResponseWriter, statusCode int, err error, msg string) {
	http.Error(w, fmt.Sprintf("%s: %s", http.StatusText(statusCode), msg), statusCode)
	if s.logger != nil {
		s.logger.Errorf("%s: %v", msg, err)
	}
}
