package server

import (
	"context"
	"net/http"

	gpapi "github.com/els0r/goProbe/v4/pkg/api/goprobe"
)

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
