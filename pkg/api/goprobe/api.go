package goprobe

import (
	"time"

	"github.com/els0r/goProbe/pkg/capture/capturetypes"
)

const (
	DefaultServerAddress = "localhost:8145"
)

const (
	IfacesQueryParam = "ifaces"
)

const QueryRoute = "/_query"

type response struct {
	StatusCode int    `json:"status_code"`
	Error      string `json:"error,omitempty"`
}

const StatusRoute = "/status"

type StatusResponse struct {
	response
	LastWriteout time.Time                            `json:"last_writeout"`
	Statuses     map[string]capturetypes.CaptureStats `json:"statuses"`
}

const ConfigRoute = "/config"

type ConfigRequest struct {
}
