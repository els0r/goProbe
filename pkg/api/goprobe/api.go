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

type response struct {
	StatusCode int    `json:"status_code"`     // StatusCode: stores the HTTP status code of the response. Example: 200
	Error      string `json:"error,omitempty"` // Error: stores the error message if the request failed. Example: "interface not found"
}

// StatusRoute is the route to query the current status
const StatusRoute = "/status"

// StatusResponse is the response to a status query
type StatusResponse struct {
	response
	// LastWriteout: denotes the time when the last writeout was performed
	// Example: "2021-01-01T00:05:00Z"
	LastWriteout time.Time `json:"last_writeout"`
	// StartedAt: denotes the time when the capture manager was initialized and
	// started capturing
	// Example: "2021-01-01T00:00:00Z"
	StartedAt time.Time `json:"started_at"`
	// Statuses: stores the statistics for each interface
	Statuses capturetypes.InterfaceStats `json:"statuses"`
}

// ConfigRoute is the route to query/modify the current configuration
const ConfigRoute = "/config"

// ConfigReloadRoute is the route to trigger a config reload
const ConfigReloadRoute = "/_reload"

// ConfigResponse is the response to a config query
type ConfigResponse struct {
	response
	Ifaces config.Ifaces `json:"ifaces"` // Ifaces: stores the current configuration for each interface
}

// ConfigUpdateResponse is the response to a config update
type ConfigUpdateResponse struct {
	response
	Enabled  capturetypes.IfaceChanges `json:"enabled"`  // Enabled: stores the interfaces that were enabled. Example: ["eth0", "eth1"]
	Updated  capturetypes.IfaceChanges `json:"updated"`  // Updated: stores the interfaces that were updated. Example: ["eth2"]
	Disabled capturetypes.IfaceChanges `json:"disabled"` // Disabled: stores the interfaces that were disabled. Example: ["eth5"]
}

// ConfigUpdateRequest is the payload to update the configuration of all
// interfaces stored in it
type ConfigUpdateRequest config.Ifaces
