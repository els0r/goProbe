package query

import (
	"context"

	"github.com/els0r/goProbe/pkg/results"
)

type Runner interface {
	// Run takes a query statement, executes the underlying query and returns the result(s)
	Run(ctx context.Context, args *Args) (*results.Result, error)
}
