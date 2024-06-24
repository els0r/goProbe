package api

const (
	infoPrefix = "/-"

	// HealthRoute denotes the route / URI path to the health endpoint
	HealthRoute = infoPrefix + "/health"
	// InfoRoute denotes the route / URI path to the info endpoint
	InfoRoute = infoPrefix + "/info"
	// ReadyRoute denotes the route / URI path to the ready endpoint
	ReadyRoute = infoPrefix + "/ready"
)

const (
	// QueryRoute is the route to run a goquery query
	QueryRoute = "/_query"

	// ValidationRoute is the route to validate a goquery query
	ValidationRoute = QueryRoute + "/validate"

	// SSEQueryRoute runs a goquery query with a return channel for partial results
	SSEQueryRoute = QueryRoute + "/sse"
)
