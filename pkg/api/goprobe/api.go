package goprobe

import (
	"time"

	"github.com/els0r/goProbe/pkg/goprobe/types"
)

const (
	DefaultServerAddress = "localhost:8145"
)

const QueryRoute = "/_query"

type response struct {
	StatusCode int    `json:"status_code"`
	Error      string `json:"error,omitempty"`
}

const FlowsRoute = "/flows"

type FlowsResponse struct {
	response
	Flows map[string]types.FlowInfos `json:"flows"`
}

const StatusRoute = "/status"

type StatusResponse struct {
	response
	LastWriteout time.Time                        `json:"last_writeout"`
	Statuses     map[string]types.InterfaceStatus `json:"statuses"`
}

const ConfigRoute = "/config"

type ConfigRequest struct {
}
