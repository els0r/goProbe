package distributed

import (
	"context"

	"github.com/els0r/goProbe/v4/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
)

// Querier provides a general interface for all query executors
type Querier interface {
	// Query runs the distributed query on the provided hosts and returns a channel from
	// which the results can be read. In addition, keepalives are sent via a second channel.
	// It is the responsibility of the implementing type to close the channels.
	// This may become a requirement through the interface definitions in future versions.
	Query(ctx context.Context, hosts hosts.Hosts, args *query.Args) (<-chan *results.Result, <-chan struct{})
}

// QuerierAnyable extends a "common" Querier with the support to retrieve a list of all hosts / targets
// available to the Querier
type QuerierAnyable interface {
	// AllHosts returns a list of all hosts / targets available to the Querier
	AllHosts() (hosts.Hosts, error)
}

// ErrorRunner is used to propagate an error all the way to the aggregation routine
type ErrorRunner struct {
	err error
}

// NewErrorRunner creates a new error runner
func NewErrorRunner(err error) *ErrorRunner {
	return &ErrorRunner{err: err}
}

// Run doesn't execute anything but returns the error that was passed to the constructor
func (e *ErrorRunner) Run(_ context.Context, _ *query.Args) (*results.Result, error) {
	return nil, e.err
}
