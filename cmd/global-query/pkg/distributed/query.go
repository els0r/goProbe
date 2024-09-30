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
	"github.com/els0r/telemetry/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// QueryRunner denotes a query runner / executor, wrapping a Querier interface instance with
// other fields required to perform a distributed query
type QueryRunner struct {
	resolver hosts.Resolver
	querier  Querier

	// onResult provides an event-based callback function to be executed upon reception
	// of a new result from one of the queried hosts (allowing for dynamic / iterative
	// handling of said results)
	onResult func(*results.Result) error
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

// SetResultReceivedFn registers a callback to be executed for every results.Result that is
// read off the results channel
func (q *QueryRunner) SetResultReceivedFn(f func(*results.Result) error) *QueryRunner {
	q.onResult = f
	return q
}

// ResultReceived calls the result callback with res and
func (q *QueryRunner) ResultReceived(res *results.Result) error {
	if q.onResult == nil {
		return errors.New("no event callback provided (onResult)")
	}
	return q.onResult(res)
}

// Run executes / runs the query and creates the final result structure
func (q *QueryRunner) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	// use a copy of the arguments, since some fields are modified by the querier
	queryArgs := *args

	// a distributed query, by definition, requires a list of hosts to query
	if queryArgs.QueryHosts == "" {
		return nil, fmt.Errorf("couldn't prepare query: list of target hosts is empty")
	}

	ctx, span := tracing.Start(ctx, "(*distributed.QueryRunner).Run", trace.WithAttributes(attribute.String("args", queryArgs.ToJSONString())))
	defer span.End()

	// sanitize the query attributes
	queryArgs.Query = types.SanitizeQueryType(queryArgs.Query)

	// check if the statement can be created
	stmt, err := queryArgs.Prepare()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query statement: %w", err)
	}

	// safeguards against loading too much data, as in, dumping whole
	// DBs via the network
	err = queryArgs.CheckUnboundedQueries()
	if err != nil {
		return nil, err
	}

	hostList, err := q.prepareHostList(ctx, args.QueryHosts)
	if err != nil {
		return nil, err // prepareHostList() returns formatted error
	}

	// log the query
	logger := logging.Logger().With("hosts", hostList)

	logger.Info("reading query results from querier")

	finalResult := aggregateResults(ctx, stmt,
		q.querier.Query(ctx, hostList, &queryArgs), q.onResult,
	)

	finalResult.End()

	// truncate results based on the limit
	if queryArgs.NumResults < uint64(len(finalResult.Rows)) {
		finalResult.Rows = finalResult.Rows[:queryArgs.NumResults]
	}
	finalResult.Summary.Hits.Displayed = len(finalResult.Rows)

	return finalResult, nil
}

func (q *QueryRunner) prepareHostList(ctx context.Context, queryHosts string) (hostList hosts.Hosts, err error) {
	ctx, span := tracing.Start(ctx, "(*distributed.QueryRunner).prepareHostList", trace.WithAttributes(attribute.String("hosts", queryHosts)))
	defer span.End()

	// Handle ANY (all hosts) case
	if types.IsAnySelector(queryHosts) {
		if querierAnyable, ok := q.querier.(QuerierAnyable); ok {
			if hostList, err = querierAnyable.AllHosts(); err != nil {
				err = fmt.Errorf("failed to extract list of all hosts: %w", err)
			}
		} else {
			err = errors.New("querier type does not support querying all hosts")
		}

		return
	}

	// Default handling via resolver
	if hostList, err = q.resolver.Resolve(ctx, queryHosts); err != nil {
		err = fmt.Errorf("failed to resolve host list: %w", err)
	}

	return
}

// aggregateResults takes finished query workloads from the workloads channel, aggregates the result by merging the rows and summaries,
// and returns the final result. The `tracker` variable provides information about potential Run failures for individual hosts
func aggregateResults(ctx context.Context, stmt *query.Statement, queryResults <-chan *results.Result, onResult func(*results.Result) error) (finalResult *results.Result) {
	ctx, span := tracing.Start(ctx, "aggregateResults")
	defer span.End()

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
			if qr.Err() != nil {
				// unwrap the error if it's possible
				uerr := errors.Unwrap(qr.Err())
				if uerr == nil {
					uerr = qr.Err()
				}

				finalResult.HostsStatuses.SetErr(qr.Hostname, uerr)

				logger.Error(qr.Err())
				continue
			}

			res := qr

			for host, status := range res.HostsStatuses {
				finalResult.HostsStatuses[host] = status
			}

			// for the final result, the hostname is only set if the result was from a single host
			if len(finalResult.HostsStatuses) > 0 {
				res.Hostname = ""
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
			finalResult.Summary.Totals.Add(res.Summary.Totals)
			finalResult.Summary.Stats.Add(res.Summary.Stats)

			// take the total from the query result. Since there may be overlap between the queries of two
			// different systems, the overlap has to be deducted from the total
			finalResult.Summary.Hits.Total += res.Summary.Hits.Total - merged

			if onResult != nil {
				// make sure the rows are set for the results callback
				if len(rowMap) > 0 {
					finalResult.Rows = rowMap.ToRowsSorted(results.By(stmt.SortBy, stmt.Direction, stmt.SortAscending))
				}
				err := onResult(finalResult)
				if err != nil {
					logger.With("error", err).Error("failed to call results callback")
				}
			}
		}
	}
}
