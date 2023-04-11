package distributed

import (
	"context"
	"fmt"
	"os"

	"github.com/els0r/goProbe/pkg/api/goprobe/client"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"gopkg.in/yaml.v3"
)

type QuerierType string

const (
	APIClientQuerierType QuerierType = "api"
)

type Querier interface {
	CreateQueryWorkload(ctx context.Context, host string, args *query.Args) (*QueryWorkload, error)
}

type APIClientQuerier struct {
	apiEndpoints map[string]*client.Config
}

func NewAPIClientQuerier(cfgPath string) (*APIClientQuerier, error) {
	a := &APIClientQuerier{
		apiEndpoints: make(map[string]*client.Config),
	}

	// read in the endpoints config
	f, err := os.Open(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config: %w", err)
	}
	defer f.Close()

	err = yaml.NewDecoder(f).Decode(a.apiEndpoints)
	if err != nil {
		return nil, fmt.Errorf("failed to read in config: %w", err)
	}
	return a, nil
}

func (a *APIClientQuerier) CreateQueryWorkload(ctx context.Context, host string, args *query.Args) (*QueryWorkload, error) {
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

func (e *errorRunner) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	return nil, e.err
}
