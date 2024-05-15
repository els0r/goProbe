package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
)

func (server *Server) getIfaceConfigHandler() func(ctx context.Context, input *GetIfaceConfigInput) (*GetConfigOutput, error) {
	return func(ctx context.Context, input *GetIfaceConfigInput) (*GetConfigOutput, error) {
		output := &GetConfigOutput{}
		resp := &gpapi.ConfigResponse{}
		output.Body = resp

		resp.StatusCode = http.StatusOK

		// query single interface if path parameter was supplied
		if input.Iface != "" {
			resp.Ifaces = server.captureManager.Config(input.Iface)
		}
		if len(resp.Ifaces) == 0 {
			resp.StatusCode = http.StatusNoContent
		}

		output.Status = resp.StatusCode

		return output, nil
	}
}

func (server *Server) getIfacesConfigHandler() func(ctx context.Context, input *GetIfacesConfigInput) (*GetConfigOutput, error) {
	return func(ctx context.Context, input *GetIfacesConfigInput) (*GetConfigOutput, error) {
		output := &GetConfigOutput{}
		resp := &gpapi.ConfigResponse{}
		output.Body = resp

		resp.StatusCode = http.StatusOK

		// query multiple if supplied in query parameters
		if len(input.Ifaces) > 0 {
			resp.Ifaces = server.captureManager.Config(input.Ifaces...)
		} else {
			// fetch all
			resp.Ifaces = server.captureManager.Config()
		}
		if len(resp.Ifaces) == 0 {
			resp.StatusCode = http.StatusNoContent
		}

		output.Status = resp.StatusCode

		return output, nil
	}
}

func (server *Server) putConfigHandler() func(context.Context, *PutConfigInput) (*ConfigUpdateOutput, error) {
	return func(ctx context.Context, input *PutConfigInput) (*ConfigUpdateOutput, error) {
		output := &ConfigUpdateOutput{}
		resp := &gpapi.ConfigUpdateResponse{}
		output.Body = resp

		resp.StatusCode = http.StatusOK

		err := input.Body.Validate()
		if err != nil {
			return output, huma.Error422UnprocessableEntity("config validation failed", err)
		}

		server.configMonitor.PutIfaceConfig(input.Body)
		resp.Enabled, resp.Updated, resp.Disabled, err = server.configMonitor.Apply(ctx, server.captureManager.Update)
		if err != nil {
			resp.StatusCode = http.StatusBadRequest
			resp.Error = err.Error()

			return output, huma.Error400BadRequest("config update failed", err)
		}

		return output, nil
	}
}

func (server *Server) reloadConfigHandler() func(context.Context, *struct{}) (*ConfigUpdateOutput, error) {
	return func(ctx context.Context, _ *struct{}) (*ConfigUpdateOutput, error) {
		output := &ConfigUpdateOutput{}
		resp := &gpapi.ConfigUpdateResponse{}
		output.Body = resp

		resp.StatusCode = http.StatusOK

		var err error
		resp.Enabled, resp.Updated, resp.Disabled, err = server.configMonitor.Reload(ctx, server.captureManager.Update)
		if err != nil {
			resp.StatusCode = http.StatusInternalServerError
			resp.Error = err.Error()

			return output, huma.Error500InternalServerError("config update failed", err)
		}

		return output, nil
	}
}
