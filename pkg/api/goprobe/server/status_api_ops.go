package server

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
)

var statusTags = []string{"Status"}

const (
	getStatusOpName = "get-status"

	getStatusSingle   = getStatusOpName + "-single"
	getStatusMultiple = getStatusOpName + "-multiple"
)

func (server *Server) registerStatusAPI() {
	huma.Register(server.API(),
		huma.Operation{
			OperationID: getStatusSingle,
			Method:      http.MethodGet,
			Path:        gpapi.StatusRoute + "/{iface}",
			Summary:     "Get capture status (single)",
			Description: "Gets capture status of a single interface",
			Tags:        statusTags,
		},
		server.getIfaceStatusHandler(),
	)
	huma.Register(server.API(),
		huma.Operation{
			OperationID: getStatusMultiple,
			Method:      http.MethodGet,
			Path:        gpapi.StatusRoute,
			Summary:     "Get capture status (many)",
			Description: "Gets capture status of one or more (or all) interfaces",
			Tags:        statusTags,
		},
		server.getIfacesStatusHandler(),
	)
}

// GetIfaceStatusInput describes the input to a status request for a single interface
type GetIfaceStatusInput struct {
	Iface string `path:"iface" doc:"Interface to get status from" minLength:"2"`
}

// GetIfacesStatusInput describes the input to a status request
type GetIfacesStatusInput struct {
	Ifaces []string `query:"ifaces" doc:"Interfaces to get status from" required:"false" minItems:"1"`
}

// GetStatusOutput returns the status fetched during a status request
type GetStatusOutput struct {
	Status int
	Body   *gpapi.StatusResponse
}
