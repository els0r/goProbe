package distributed

import (
	"context"
	"fmt"
	"os"

	"github.com/els0r/goProbe/pkg/api/goprobe/client"
	"github.com/els0r/goProbe/pkg/query"
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
		return nil, fmt.Errorf("couldn't find endpoint configuration for host")
	}
	qw.Runner = client.NewFromConfig(cfg)

	return qw, nil
}
