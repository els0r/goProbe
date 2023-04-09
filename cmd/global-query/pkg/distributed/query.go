package distributed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
)

type Querier struct {
	resolver hosts.Resolver
	querier  hosts.Querier
}

func NewQuerier(resolver hosts.Resolver, querier hosts.Querier) *Querier {
	return &Querier{
		resolver: resolver,
		querier:  querier,
	}
}

func (q *Querier) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	// use a copy of the arguments, since some fields are modified by the querier
	queryArgs := *args

	// a distributed query, by definition, requires a list of hosts to query
	if queryArgs.HostQuery == "" {
		return nil, fmt.Errorf("couldn't prepare query: list of target hosts is empty")
	}

	// check if the statement can be created
	stmt, err := queryArgs.Prepare()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query statement: %w", err)
	}

	hostList, err := q.resolver.Resolve(ctx, queryArgs.HostQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve host list: %v", err)
	}

	// log the query
	logger := logging.Logger().With("hosts", hostList)

	b, err := json.Marshal(queryArgs)
	if err == nil {
		logger = logger.With("query", string(b))
	}
	logger.Info("running query")

	// query pipeline setup
	// sets up a fan-out, fan-in query processing pipeline
	numRunners := len(hostList)

	finalResult := hosts.AggregateResults(ctx, stmt,
		hosts.RunQueries(ctx, numRunners,
			hosts.PrepareQueries(ctx, q.querier, hostList, &queryArgs),
		),
	)

	// truncate results based on the limit
	finalResult.End()

	if queryArgs.NumResults < len(finalResult.Rows) {
		finalResult.Rows = finalResult.Rows[:queryArgs.NumResults]
	}
	finalResult.Summary.Hits.Displayed = len(finalResult.Rows)

	return finalResult, nil
}
