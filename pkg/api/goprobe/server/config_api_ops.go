package server

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/v4/cmd/goProbe/config"
	gpapi "github.com/els0r/goProbe/v4/pkg/api/goprobe"
)

var configTags = []string{"Config"}

const (
	getConfigOpName    = "get-config"
	reloadConfigOpName = "reload-config"
	updateConfigOpName = "update-config"

	getConfigSingle   = getConfigOpName + "-single"
	getConfigMultiple = getConfigOpName + "-many"
)

func (server *Server) registerConfigAPI() {
	huma.Register(server.API(),
		huma.Operation{
			OperationID: getConfigSingle,
			Method:      http.MethodGet,
			Path:        gpapi.ConfigRoute + "/{iface}",
			Summary:     "Get capture configuration (single)",
			Description: "Gets capture configuration of a single interface",
			Tags:        configTags,
		},
		server.getIfaceConfigHandler(),
	)
	huma.Register(server.API(),
		huma.Operation{
			OperationID: getConfigMultiple,
			Method:      http.MethodGet,
			Path:        gpapi.ConfigRoute,
			Summary:     "Get capture configuration (many)",
			Description: "Gets capture configuration of multiple interfaces",
			Tags:        configTags,
		},
		server.getIfacesConfigHandler(),
	)
	huma.Register(server.API(),
		huma.Operation{
			OperationID: updateConfigOpName,
			Method:      http.MethodPut,
			Path:        gpapi.ConfigRoute,
			Summary:     "Update capture configuration",
			Description: "Updates the capture configuration for all interfaces",
			Tags:        configTags,
		},
		server.putConfigHandler(),
	)
	huma.Register(server.API(),
		huma.Operation{
			OperationID: reloadConfigOpName,
			Method:      http.MethodPost,
			Path:        gpapi.ConfigRoute + gpapi.ConfigReloadRoute,
			Summary:     "Reload capture configuration",
			Description: "Reloads the capture configuration for all interfaces",
			Tags:        configTags,
		},
		server.reloadConfigHandler(),
	)
}

// GetIfaceConfigInput describes the input to a config request for a single interface
type GetIfaceConfigInput struct {
	Iface string `path:"iface" doc:"Interface to get configuration from" minLength:"2"`
}

// GetIfacesConfigInput describes the input to a config request for multiple interfaces
type GetIfacesConfigInput struct {
	Ifaces []string `query:"ifaces" doc:"Interfaces to get configuration from" required:"false" minItems:"1"`
}

// GetConfigOutput returns the interface config(s) fetched during a config request
type GetConfigOutput struct {
	Status int
	Body   *gpapi.ConfigResponse
}

// PutConfigInput is the input to a config update
type PutConfigInput struct {
	Body config.Ifaces
}

// ConfigUpdateOutput returns the updated interfaces
type ConfigUpdateOutput struct {
	Body *gpapi.ConfigUpdateResponse
}
