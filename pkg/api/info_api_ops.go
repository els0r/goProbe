package api

import (
	"context"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/v4/pkg/version"
)

var infoTags = []string{"Info"}

const (
	healthy = "healthy"
	ready   = "ready"

	getHealthOpName = "get-health"
	getInfoOpName   = "get-info"
	getReadyOpName  = "get-ready"
)

// ServiceInfo summarizes the running service's name, version, and commit. If running in
// kubernetes, it will also print the name of the pod which returned the API call
type ServiceInfo struct {
	Name    string `json:"name" doc:"Service name" example:"global-query"`                                                // Name: service name
	Version string `json:"version" doc:"Service (semantic) version" example:"4.0.0-824f5847"`                             // Version: (semantic) version and commit short
	Commit  string `json:"commit,omitempty" doc:"Full git commit SHA" example:"824f58479a8f326cb350085b3a0e287645e11bc1"` // Commit: full git commit SHA
	Pod     string `json:"pod,omitempty" doc:"Name of kubernetes pod (if available)"`                                     // Pod: name of kubernetes pod, if available
}

// GetInfoOutput is the output of the info request
type GetInfoOutput struct {
	Body struct {
		*ServiceInfo
	}
}

// GetInfoOperation is the operation for getting a greeting.
func GetInfoOperation() huma.Operation {
	return huma.Operation{
		OperationID: getInfoOpName,
		Method:      http.MethodGet,
		Path:        InfoRoute,
		Summary:     "Get application info",
		Description: "Get runtime information about the application.",
		Tags:        infoTags,
	}
}

// GetServiceInfoHandler returns a huma compatible handler that returns the service name, version, and commit
func GetServiceInfoHandler(serviceName string) func(context.Context, *struct{}) (*GetInfoOutput, error) {
	info := &ServiceInfo{
		Name:    serviceName,
		Version: version.Short(),
		Commit:  version.GitSHA,
	}

	// try to ascertain the running pod's name
	for _, env := range []string{"POD_NAME", "POD", "PODNAME"} {
		podName := os.Getenv(env)
		if podName != "" {
			info.Pod = podName
			break
		}
	}

	return func(ctx context.Context, _ *struct{}) (*GetInfoOutput, error) {
		output := &GetInfoOutput{}
		output.Body.ServiceInfo = info
		return output, nil
	}
}

// GetHealthOutput returns the output of the health command
type GetHealthOutput struct {
	Body struct {
		Status string `json:"status" doc:"Health status of application" example:"healthy"`
	}
}

// GetHealthOperation is the operation for getting the health of the app
func GetHealthOperation() huma.Operation {
	return huma.Operation{
		OperationID: getHealthOpName,
		Method:      http.MethodGet,
		Path:        HealthRoute,
		Summary:     "Get application health",
		Description: "Get info whether the application is running.",
		Tags:        infoTags,
	}
}

// GetHealthHandler returns a handler that returns the application readiness state
func GetHealthHandler() func(context.Context, *struct{}) (*GetHealthOutput, error) {
	return func(ctx context.Context, _ *struct{}) (*GetHealthOutput, error) {
		output := &GetHealthOutput{}
		output.Body.Status = healthy
		return output, nil
	}
}

// GetReadyOutput returns the output of the ready command
type GetReadyOutput struct {
	Body struct {
		Status string `json:"status" doc:"Ready status of application" example:"ready"`
	}
}

// GetReadyOperation is the operation for getting the ready state
func GetReadyOperation() huma.Operation {
	return huma.Operation{
		OperationID: getReadyOpName,
		Method:      http.MethodGet,
		Path:        ReadyRoute,
		Summary:     "Get application readiness",
		Description: "Get info whether the application is ready.",
		Tags:        infoTags,
	}
}

// GetReadyHandler returns a handler that returns the application readiness state
func GetReadyHandler() func(context.Context, *struct{}) (*GetReadyOutput, error) {
	return func(ctx context.Context, _ *struct{}) (*GetReadyOutput, error) {
		output := &GetReadyOutput{}
		output.Body.Status = ready
		return output, nil
	}
}
