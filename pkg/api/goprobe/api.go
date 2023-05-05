package goprobe

import (
	"time"

	"github.com/els0r/goProbe/pkg/goprobe/types"
)

const (
	DefaultServerAddress = "localhost:8145"
)

// Routes
const (
	QueryRoute = "/_query"

	StatusRoute = "/status"
)

type response struct {
	StatusCode int    `json:"status_code"`
	Error      string `json:"error,omitempty"`
}

type StatusResponse struct {
	response
	LastWriteout time.Time                        `json:"last_writeout"`
	Statuses     map[string]types.InterfaceStatus `json:"statuses"`
}
