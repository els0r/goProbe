package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"
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

	// register routes specific to distributed querying
	dqr, ok := querier.(SSEQueryRunner)
	if ok {
		registerDistributedQueryAPI(a, caller, dqr, middlewares)
		return
	}

	// query running in case it isn't a distributed API (e.g. goProbe's query endpoint)
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

func registerDistributedQueryAPI(a huma.API, caller string, qr SSEQueryRunner, middlewares huma.Middlewares) {
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
		getBodyQueryRunnerHandler(caller, qr),
	)
	sse.Register(a,
		huma.Operation{
			OperationID: "query-post-run-sse",
			Method:      http.MethodPost,
			Path:        SSEQueryRoute,
			Summary:     "Run query with server sent events (SSE)",
			Description: "Runs a query based on the parameters provided in the body. Pushes back partial results via SSE",
			Middlewares: middlewares,
			Tags:        queryTags,
		},
		map[string]any{
			string(StreamEventQueryError):    &query.DetailError{},
			string(StreamEventPartialResult): &PartialResult{},
			string(StreamEventFinalResult):   &FinalResult{},
			string(StreamEventKeepalive):     &Keepalive{},
		},
		getSSEBodyQueryRunnerHandler(caller, qr),
	)
}

// StreamEventType describes the type of server sent event
type StreamEventType string

// Different event types that the query server sends
const (
	StreamEventQueryError    StreamEventType = "queryError"
	StreamEventPartialResult StreamEventType = "partialResult"
	StreamEventFinalResult   StreamEventType = "finalResult"
	StreamEventKeepalive     StreamEventType = "keepalive"
)

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

// PartialResult represents an update to the results structure. It SHOULD only be used if the
// results.Result object will be further modified / aggregated. This data structure is relevant
// only in the context of SSE
type PartialResult struct{ *results.Result }

// FinalResult represents the result which is sent after all aggregation of partial results has
// completed. It SHOULD only be sent at the end of a streaming operation. This data structure is relevant
// only in the context of SSE
type FinalResult struct{ *results.Result }

// Keepalive represents an keeplive signal. This data structure is relevant
// only in the context of SSE
type Keepalive struct{}
