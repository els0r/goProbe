package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/cmd/goProbe/config"
	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
)

var configTags = []string{"Config"}

func (server *Server) registerConfigAPI() {
	huma.Register(server.API(),
		huma.Operation{
			OperationID: "get-config-single",
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
			OperationID: "get-config-many",
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
			OperationID: "update-config",
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
			OperationID: "reload-config",
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

// PutConfigInput is the input to a config update
type PutConfigInput struct {
	Body config.Ifaces
}

// ConfigUpdateOutput returns the updated interfaces
type ConfigUpdateOutput struct {
	Body *gpapi.ConfigUpdateResponse
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
