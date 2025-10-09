package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/stretchr/testify/require"
)

// noopRunner implements query.Runner and is used to register the API without SSE support
type noopRunner struct{}

func (noopRunner) Run(_ context.Context, _ *query.Args) (*results.Result, error) { //nolint:revive // test stub
	return nil, nil
}

// sseRunner implements SSEQueryRunner to trigger distributed validation variants
type sseRunner struct{}

func (sseRunner) Run(_ context.Context, _ *query.Args) (*results.Result, error) { //nolint:revive // test stub
	return nil, nil
}

func (sseRunner) RunStreaming(_ context.Context, _ *query.Args, _ sse.Sender) (*results.Result, error) { //nolint:revive // test stub
	return nil, nil
}

func setupAPI(t *testing.T, runner any) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)

	// middlewares are not relevant for validation
	RegisterQueryAPI(api, "test", runner.(query.Runner), nil)
	return api
}

func TestValidatePOST_Valid(t *testing.T) {
	api := setupAPI(t, noopRunner{})

	body := query.Args{Query: "sip", Ifaces: "eth0", Format: "json"}
	resp := api.Post(ValidationRoute, "Content-Type: application/json", body)

	require.Equal(t, http.StatusNoContent, resp.Code)
	require.Empty(t, resp.Body.String())
}

func TestValidatePOST_InvalidArgs_Returns422(t *testing.T) {
	api := setupAPI(t, noopRunner{})

	// Missing ifaces should trigger validation error; include valid format
	body := query.Args{Query: "sip", Ifaces: "", Format: "json"}
	resp := api.Post(ValidationRoute, "Content-Type: application/json", body)

	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	// basic smoke checks on problem details
	require.Contains(t, resp.Header().Get("Content-Type"), "application/problem+json")
	require.Contains(t, resp.Body.String(), "body.ifaces")
}

func TestValidateGET_Valid(t *testing.T) {
	api := setupAPI(t, noopRunner{})

	resp := api.Get(ValidationRoute + "?query=sip&ifaces=eth0&format=json")

	require.Equal(t, http.StatusNoContent, resp.Code)
	require.Empty(t, resp.Body.String())
}

func TestValidatePOST_Distributed_MissingHosts_Returns422(t *testing.T) {
	// Use SSE-capable runner to register distributed validation (requires query_hosts)
	_, api := humatest.New(t)
	RegisterQueryAPI(api, "test", sseRunner{}, nil)

	body := query.Args{Query: "sip", Ifaces: "eth0", Format: "json"}
	resp := api.Post(ValidationRoute, "Content-Type: application/json", body)

	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	require.Contains(t, resp.Body.String(), "Empty host query")
	require.Contains(t, resp.Body.String(), "body.query_hosts")
}

func TestValidatePOST_Distributed_WithHosts_Valid(t *testing.T) {
	_, api := humatest.New(t)
	RegisterQueryAPI(api, "test", sseRunner{}, nil)

	body := query.Args{Query: "sip", Ifaces: "eth0", Format: "json", QueryHosts: "hostA"}
	resp := api.Post(ValidationRoute, "Content-Type: application/json", body)

	require.Equal(t, http.StatusNoContent, resp.Code)
}

func TestValidatePOST_Distributed_WithHosts_InvalidHostSyntax(t *testing.T) {
	_, api := humatest.New(t)
	RegisterQueryAPI(api, "test", sseRunner{}, nil)

	body := query.Args{Query: "sip", Ifaces: "eth0", Format: "json", QueryHosts: "hostA	       "}
	resp := api.Post(ValidationRoute, "Content-Type: application/json", body)

	require.Equal(t, http.StatusNoContent, resp.Code)
}

func TestValidateGET_Distributed_MissingHosts_Returns422(t *testing.T) {
	_, api := humatest.New(t)
	RegisterQueryAPI(api, "test", sseRunner{}, nil)

	resp := api.Get(ValidationRoute + "?query=sip&ifaces=eth0&format=json")

	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	require.Contains(t, resp.Body.String(), "Empty host query")
}
