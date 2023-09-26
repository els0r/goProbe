package distributed

import (
	"context"

	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
)

// QuerierType denotes the type of the querier instance
type QuerierType string

const (
	// APIClientQuerierType provides the name for the goProbe API-based querier
	APIClientQuerierType QuerierType = "api"
)

// Querier provides a general interface for all query executors
type Querier interface {
	// Query runs the distributed query on the provided hosts and returns a channel from
	// which the results can be read. It is the responsibility of the implementing type
	// to close the channel.
	// This may become a requirement through the interface definitions in future versions
	Query(ctx context.Context, hosts hosts.Hosts, args *query.Args) <-chan *results.DistributedResult
}

// errorRunner is used to propagate an error all the way to the aggregation routine
type errorRunner struct {
	err error
}

func (e *errorRunner) Run(_ context.Context, _ *query.Args) (*results.Result, error) {
	return nil, e.err
}
