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
