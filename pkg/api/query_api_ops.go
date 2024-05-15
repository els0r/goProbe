package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
)

var queryTags = []string{"Query"}

// RegisterQueryAPI registers all query related endpoints
func RegisterQueryAPI(a huma.API, caller string, querier query.Runner, middlewares huma.Middlewares) {
	// validation
	huma.Register(a,
		huma.Operation{
			OperationID: "query-post-validate",
			Method:      http.MethodPost,
			Path:        ValidationRoute,
			Summary:     "Validate query parameters",
			Description: "Validates query parameters (1) for integrity (2) attempting to prepare a query statement from them",
			Tags:        queryTags,
		},
		getBodyValidationHandler(),
	)
	huma.Register(a,
		huma.Operation{
			OperationID: "query-get-validate",
			Method:      http.MethodGet,
			Summary:     "Validate query parameters",
			Path:        ValidationRoute,
			Description: "Validates query parameters (1) for integrity (2) attempting to prepare a query statement from them",
			Tags:        queryTags,
		},
		getParamsValidationHandler(),
	)
	// query running
	huma.Register(a,
		huma.Operation{
			OperationID: "query-post-run",
			Method:      http.MethodPost,
			Path:        QueryRoute,
			Summary:     "Run query",
			Description: "Runs a query based on the parameters provided in the body",
			Middlewares: middlewares,
			Tags:        queryTags,
		},
		getBodyQueryRunnerHandler(caller, querier),
	)
}

// ArgsBodyInput stores the query args to be validated in the body
type ArgsInput struct {
	Body *query.Args
}

// ArgsParamsInput stores the query args to be validated in the query parameters
type ArgsParamsInput struct {
	// for get parameters
	query.Args
	query.DNSResolution
}

// QueryResultOutput stores the result of a query
type QueryResultOutput struct {
	Body *results.Result
}
