package distributed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/els0r/goProbe/pkg/api/goprobe/client"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"gopkg.in/yaml.v3"
)

// QuerierType denotes the type of the querier instance
type QuerierType string

const (

	// APIClientQuerierType provides the name for the API-based querier
	APIClientQuerierType QuerierType = "api"
)

// Querier provides a general interface for all query executors
type Querier interface {

	// CreateQueryWorkload prepares and executes the workload required to perform the query
	CreateQueryWorkload(ctx context.Context, host string, args *query.Args) (*QueryWorkload, error)
}

// APIClientQuerier implements an API-based querier, fulfilling the Querier interface
type APIClientQuerier struct {
	apiEndpoints map[string]*client.Config
}

// NewAPIClientQuerier instantiates a new API-based querier
func NewAPIClientQuerier(cfgPath string) (*APIClientQuerier, error) {
	a := &APIClientQuerier{
		apiEndpoints: make(map[string]*client.Config),
	}

	// read in the endpoints config
	f, err := os.Open(filepath.Clean(cfgPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open config: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	err = yaml.NewDecoder(f).Decode(a.apiEndpoints)
	if err != nil {
		return nil, fmt.Errorf("failed to read in config: %w", err)
	}
	return a, nil
}

// CreateQueryWorkload prepares and executes the workload required to perform the query
func (a *APIClientQuerier) CreateQueryWorkload(_ context.Context, host string, args *query.Args) (*QueryWorkload, error) {
	qw := &QueryWorkload{
		Host: host,
		Args: args,
	}
	// create the api client runner by looking up the endpoint config for the given host
	cfg, exists := a.apiEndpoints[host]
	if !exists {
		err := fmt.Errorf("couldn't find endpoint configuration for host")

		// inject an error runner so that the workload creation error is transported into the final
		// result
		qw.Runner = &errorRunner{err: err}
	} else {
		qw.Runner = client.NewFromConfig(cfg)
	}

	return qw, nil
}

// errorRunner is used to propagate an error all the way to the aggregation routine
type errorRunner struct {
	err error
}

func (e *errorRunner) Run(_ context.Context, _ *query.Args) (*results.Result, error) {
	return nil, e.err
}
