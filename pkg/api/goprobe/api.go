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

// Response stores the HTTP status code and error detail of the response
type Response struct {
	// StatusCode: stores the HTTP status code of the response
	StatusCode int `json:"status_code" doc:"HTTP status code of the response" example:"200"`
	// Error: stores the error message if the request failed
	Error string `json:"error,omitempty" doc:"Error message if request failed" example:"interface not found"`
}

// StatusRoute is the route to query the current status
const StatusRoute = "/status"

// StatusResponse is the response to a status query
type StatusResponse struct {
	Response
	// LastWriteout: denotes the time when the last writeout was performed
	LastWriteout time.Time `json:"last_writeout" doc:"Time when the last writeout was performed" example:"2021-01-01T00:05:00Z"`
	// StartedAt: denotes the time when the capture manager was initialized and started capturing
	StartedAt time.Time `json:"started_at" doc:"Time when the capture manager was initialized and started capturing" example:"2021-01-01T00:00:00Z"`
	// Statuses: stores the statistics for each interface
	Statuses capturetypes.InterfaceStats `json:"statuses" doc:"Stores the statistics for each interface"`
}

// ConfigRoute is the route to query/modify the current configuration
const ConfigRoute = "/config"

// ConfigReloadRoute is the route to trigger a config reload
const ConfigReloadRoute = "/_reload"

// ConfigResponse is the response to a config query
type ConfigResponse struct {
	Response
	Ifaces config.Ifaces `json:"ifaces"` // Ifaces: stores the current configuration for each interface
}

// ConfigUpdateResponse is the response to a config update
type ConfigUpdateResponse struct {
	Response
	// Enabled: stores the interfaces that were enabled. Example: ["eth0", "eth1"]
	Enabled capturetypes.IfaceChanges `json:"enabled" doc:"Interfaces that were enabled"`
	// Updated: stores the interfaces that were updated. Example: ["eth2"]
	Updated capturetypes.IfaceChanges `json:"updated" doc:"Interfaces that were updated"`
	// Disabled: stores the interfaces that were disabled. Example: ["eth5"]
	Disabled capturetypes.IfaceChanges `json:"disabled" doc:"Interfaces that were disabled"`
}

// ConfigUpdateRequest is the payload to update the configuration of all
// interfaces stored in it
type ConfigUpdateRequest config.Ifaces
