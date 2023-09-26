package distributed

import (
	"context"
	"errors"
	"fmt"

	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/telemetry/logging"
)

// QueryRunner denotes a query runner / executor, wrapping a Querier interface instance with
// other fields required to perform a distributed query
type QueryRunner struct {
	resolver hosts.Resolver
	querier  Querier
}

// QueryOption configures the query runner
type QueryOption func(*QueryRunner)

// NewQueryRunner instantiates a new distributed query runner
func NewQueryRunner(resolver hosts.Resolver, querier Querier, opts ...QueryOption) (qr *QueryRunner) {
	qr = &QueryRunner{
		resolver: resolver,
		querier:  querier,
	}
	for _, opt := range opts {
		opt(qr)
	}
	return
}

// Run executes / runs the query and creates the final result structure
func (q *QueryRunner) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	// use a copy of the arguments, since some fields are modified by the querier
	queryArgs := *args

	// a distributed query, by definition, requires a list of hosts to query
	if queryArgs.QueryHosts == "" {
		return nil, fmt.Errorf("couldn't prepare query: list of target hosts is empty")
	}

	// check if the statement can be created
	stmt, err := queryArgs.Prepare()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query statement: %w", err)
	}

	hostList, err := q.resolver.Resolve(ctx, queryArgs.QueryHosts)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve host list: %w", err)
	}

	// log the query
	logger := logging.Logger().With("hosts", hostList)

	logger.Info("reading query results from querier")

	finalResult := aggregateResults(ctx, stmt,
		q.querier.Query(ctx, hostList, &queryArgs),
	)

	finalResult.End()

	// truncate results based on the limit
	if queryArgs.NumResults < uint64(len(finalResult.Rows)) {
		finalResult.Rows = finalResult.Rows[:queryArgs.NumResults]
	}
	finalResult.Summary.Hits.Displayed = len(finalResult.Rows)

	return finalResult, nil
}

// aggregateResults takes finished query workloads from the workloads channel, aggregates the result by merging the rows and summaries,
// and returns the final result. The `tracker` variable provides information about potential Run failures for individual hosts
func aggregateResults(ctx context.Context, stmt *query.Statement, queryResults <-chan *results.DistributedResult) (finalResult *results.Result) {
	// aggregation
	finalResult = results.New()
	finalResult.Start()

	var rowMap = make(results.RowsMap)

	// tracker maps for meta info
	var ifaceMap = make(map[string]struct{})

	logger := logging.FromContext(ctx)

	defer func() {
		if len(rowMap) > 0 {
			finalResult.Rows = rowMap.ToRowsSorted(results.By(stmt.SortBy, stmt.Direction, stmt.SortAscending))
		}
		finalResult.End()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case qr, open := <-queryResults:
			if !open {
				return
			}
			logger := logger.With("hostname", qr.Hostname)
			if qr.Error != nil {
				// unwrap the error if it's possible
				var msg string

				uerr := errors.Unwrap(qr.Error)
				if uerr != nil {
					msg = uerr.Error()
				} else {
					msg = qr.Error.Error()
				}

				finalResult.HostsStatuses[qr.Hostname] = results.Status{
					Code:    types.StatusError,
					Message: msg,
				}
				logger.Error(qr.Error)
				continue
			}

			res := qr.Result
			for host, status := range res.HostsStatuses {
				finalResult.HostsStatuses[host] = status
			}

			// merges the traffic data
			merged := rowMap.MergeRows(res.Rows)

			// merges the metadata
			for _, iface := range res.Summary.Interfaces {
				ifaceMap[iface] = struct{}{}
			}
			var ifaces = make([]string, 0, len(ifaceMap))
			for iface := range ifaceMap {
				ifaces = append(ifaces, iface)
			}

			finalResult.Summary.Interfaces = ifaces

			finalResult.Query = res.Query
			finalResult.Summary.First = res.Summary.First
			finalResult.Summary.Last = res.Summary.Last
			finalResult.Summary.Totals = finalResult.Summary.Totals.Add(res.Summary.Totals)

			// take the total from the query result. Since there may be overlap between the queries of two
			// different systems, the overlap has to be deducted from the total
			finalResult.Summary.Hits.Total += res.Summary.Hits.Total - merged
		}
	}
}
