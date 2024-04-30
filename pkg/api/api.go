package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

const (
	infoPrefix = "/-"

	// HealthRoute denotes the route / URI path to the health endpoint
	HealthRoute = infoPrefix + "/health"
	// InfoRoute denotes the route / URI path to the info endpoint
	InfoRoute = infoPrefix + "/info"
	// ReadyRoute denotes the route / URI path to the ready endpoint
	ReadyRoute = infoPrefix + "/ready"
)

// GetInfoOperation is the operation for getting a greeting.
var GetInfoOperation = huma.Operation{
	OperationID: "get-info",
	Method:      http.MethodGet,
	Path:        InfoRoute,
	Summary:     "Get application info",
	Description: "Get runtime information about the application.",
	Tags:        []string{"Info"},
}

func Register(api huma.API) {

}

const (
	// QueryRoute is the route to run a goquery query
	QueryRoute = "/_query"

	// ValidationRoute is the route to validate a goquery query
	ValidationRoute = QueryRoute + "/validate"
)
