package goprobe

import (
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
)

const (
	// DefaultServerAddress is the default address of the goProbe server
	DefaultServerAddress = "localhost:8145"
)

const (
	// IfacesQueryParam is the query parameter to specify the interfaces to query
	IfacesQueryParam = "ifaces"
)

// QueryRoute is the route to run a goquery query
const QueryRoute = "/_query"

type response struct {
	StatusCode int    `json:"status_code"`
	Error      string `json:"error,omitempty"`
}

// StatusRoute is the route to query the current status
const StatusRoute = "/status"

// StatusResponse is the response to a status query
type StatusResponse struct {
	response
	// LastWriteout denotes the time when the last writeout was performed
	LastWriteout time.Time `json:"last_writeout"`
	// StartedAt denotes the time when the capture manager was initialized and
	// started capturing
	StartedAt time.Time `json:"started_at"`
	// Statuses stores the statistics for each interface
	Statuses capturetypes.InterfaceStats `json:"statuses"`
}

// ConfigRoute is the route to query/modify the current configuration
const ConfigRoute = "/config"

// ConfigReloadRoute is the route to trigger a config reload
const ConfigReloadRoute = "/_reload"

// ConfigResponse is the response to a config query
type ConfigResponse struct {
	response
	// Ifaces stores the current configuration for each interface
	Ifaces config.Ifaces `json:"ifaces"`
}

// ConfigUpdateResponse is the response to a config update
type ConfigUpdateResponse struct {
	response
	// Enabled stores the interfaces that were enabled
	Enabled []string `json:"enabled"`
	// Updated stores the interfaces that were updated
	Updated []string `json:"updated"`
	// Disabled stores the interfaces that were disabled
	Disabled []string `json:"disabled"`
}

// ConfigUpdateRequest is the payload to update the configuration of all
// interfaces stored in it
type ConfigUpdateRequest config.Ifaces
