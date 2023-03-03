package errors

import (
	"context"
	"fmt"
	"net/http"

	"github.com/els0r/goProbe/pkg/logging"
)

// Handler can be used to determine how an API should deal with providing details
// on errors
type Handler interface {
	// Handle governs how an error is returned to the caller
	Handle(ctx context.Context, w http.ResponseWriter, statusCode int, err error, msg string)
}

// StandardHandler returns and http.Error and has an internal logger for logging
// errors. It's there to ensure that it is not only apparent that an error occurred
// but also what kind
type StandardHandler struct{}

// NewStandardHandler returns a new new standard handler.
func NewStandardHandler() *StandardHandler {
	return &StandardHandler{}
}

// Handle is a convenience method to return a standard formatted http Error and
// write a log line if a logger is provided
func (s *StandardHandler) Handle(ctx context.Context, w http.ResponseWriter, statusCode int, err error, msg string) {
	logger := logging.WithContext(ctx)

	http.Error(w, fmt.Sprintf("%s: %s", http.StatusText(statusCode), msg), statusCode)
	logger.Errorf("%s: %v", msg, err)
}
