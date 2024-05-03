package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
)

var statusTags = []string{"Status"}

func (server *Server) registerStatusAPI() {
	huma.Register(server.API(),
		huma.Operation{
			OperationID: "get-status-single",
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
			OperationID: "get-status-multiple",
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

func (server *Server) getIfaceStatusHandler() func(ctx context.Context, input *GetIfaceStatusInput) (*GetStatusOutput, error) {
	return func(ctx context.Context, input *GetIfaceStatusInput) (*GetStatusOutput, error) {
		output := &GetStatusOutput{}
		resp := &gpapi.StatusResponse{}
		output.Body = resp

		resp.StatusCode = http.StatusOK
		resp.StartedAt, resp.LastWriteout = server.captureManager.GetTimestamps()

		// query single interface if path parameter was supplied
		if input.Iface != "" {
			resp.Statuses = server.captureManager.Status(ctx, input.Iface)
		}
		if len(resp.Statuses) == 0 {
			resp.StatusCode = http.StatusNoContent
		}

		output.Status = resp.StatusCode

		return output, nil
	}
}

func (server *Server) getIfacesStatusHandler() func(ctx context.Context, input *GetIfacesStatusInput) (*GetStatusOutput, error) {
	return func(ctx context.Context, input *GetIfacesStatusInput) (*GetStatusOutput, error) {
		output := &GetStatusOutput{}
		resp := &gpapi.StatusResponse{}
		output.Body = resp

		resp.StatusCode = http.StatusOK
		resp.StartedAt, resp.LastWriteout = server.captureManager.GetTimestamps()

		// query multiple if supplied in query parameters
		if len(input.Ifaces) > 0 {
			resp.Statuses = server.captureManager.Status(ctx, input.Ifaces...)
		} else {
			// fetch all
			resp.Statuses = server.captureManager.Status(ctx)
		}
		if len(resp.Statuses) == 0 {
			resp.StatusCode = http.StatusNoContent
		}

		output.Status = resp.StatusCode

		return output, nil
	}
}
