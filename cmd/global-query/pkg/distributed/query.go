// Package distributed handles distributed queries across multiple hosts
package distributed

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/distributed"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/telemetry/logging"
	"github.com/els0r/telemetry/tracing"
	"github.com/fako1024/gotools/concurrency"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	// DefaultSemTimeout is the default / fallback amount of time to wait for acquisition
	// of a semaphore when performing concurrent queries
	DefaultSemTimeout = time.Second
	// maxLimitStreaming is the maximum number of results to return in a streaming query's partialResult
	maxLimitStreaming = 100
)

var errQuerierTypeAllNotSupported = errors.New("querier type does not support querying all hosts")

// QueryRunner denotes a query runner / executor, wrapping a Querier interface instance with
// other fields required to perform a distributed query
type QueryRunner struct {
	resolvers *hosts.ResolverMap
	querier   distributed.Querier
	sem       concurrency.Semaphore
}

// QueryOption configures the query runner
type QueryOption func(*QueryRunner)

// WithMaxConcurrent sets a maximum number of concurrent running queries
func WithMaxConcurrent(sem chan struct{}) QueryOption {
	return func(qr *QueryRunner) {
		qr.sem = sem
	}
}

// NewQueryRunner instantiates a new distributed query runner
func NewQueryRunner(resolvers *hosts.ResolverMap, querier distributed.Querier, opts ...QueryOption) (qr *QueryRunner) {
	qr = &QueryRunner{
		resolvers: resolvers,
		querier:   querier,
	}
	for _, opt := range opts {
		opt(qr)
	}
	return qr
}

// Run executes / runs the query and creates the final result structure
func (q *QueryRunner) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
	ctx, span := tracing.Start(ctx, "(*distributed.QueryRunner).Run")
	defer span.End()

	return q.run(ctx, args, nil)
}

func (q *QueryRunner) RunStreaming(ctx context.Context, args *query.Args, send sse.Sender) (*results.Result, error) {
	ctx, span := tracing.Start(ctx, "(*distributed.QueryRunner).RunStreaming")
	defer span.End()

	return q.run(ctx, args, send)
}

// Run executes / runs the query and creates the final result structure
func (q *QueryRunner) run(ctx context.Context, args *query.Args, send sse.Sender) (*results.Result, error) {
	// use a copy of the arguments, since some fields are modified by the querier
	queryArgs := *args

	// a distributed query, by definition, requires a list of hosts to query
	if queryArgs.QueryHosts == "" {
		return nil, fmt.Errorf("couldn't prepare query: query for target hosts is empty")
	}
	queryArgs.QueryHosts = types.SanitizeQueryHosts(queryArgs.QueryHosts)

	// select hosts resolver based on query configuration. Will always fall back to the strings resolver in case none
	// is selected
	resolverType := "string"
	if queryArgs.QueryHostsResolverType != "" {
		resolverType = queryArgs.QueryHostsResolverType
	}
	hostsResolver, ok := q.resolvers.Get(resolverType)
	if !ok {
		return nil, fmt.Errorf("hosts resolver type %q not available", resolverType)
	}

	ctx, span := tracing.Start(ctx, "(*distributed.QueryRunner).run", trace.WithAttributes(attribute.String("args", queryArgs.ToJSONString())))
	defer span.End()

	// sanitize the query attributes
	queryArgs.Query = types.SanitizeQueryType(queryArgs.Query)

	// check if the statement can be created
	stmt, err := queryArgs.Prepare()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query statement: %w", err)
	}

	smeDone, err := q.checkSemaphore(stmt)
	if err != nil {
		return &results.Result{
			Status: results.Status{
				Code:    types.StatusTooManyRequests,
				Message: "too many concurrent requests",
			},
		}, nil
	}
	defer smeDone()

	// safeguards against loading too much data, as in, dumping whole
	// DBs via the network
	err = queryArgs.CheckUnboundedQueries()
	if err != nil {
		return nil, err
	}

	hostList, err := q.prepareHostList(ctx, hostsResolver, args.QueryHosts)
	if err != nil {
		return nil, err // prepareHostList() returns formatted error
	}

	// log the query
	logger := logging.Logger().With("hosts", hostList)

	logger.Info("reading query results from querier")

	resChan, keepaliveChan := q.querier.Query(ctx, hostList, &queryArgs)
	if send != nil && queryArgs.KeepAlive > 0 {
		q.forwardKeepalives(ctx, keepaliveChan, send, queryArgs.KeepAlive)
	}
	finalResult := aggregateResults(ctx, stmt,
		resChan, send,
	)

	return finalResult, nil
}

func (q *QueryRunner) prepareHostList(ctx context.Context, resolver hosts.Resolver, queryHosts string) (hostList hosts.Hosts, err error) {
	ctx, span := tracing.Start(ctx, "(*distributed.QueryRunner).prepareHostList", trace.WithAttributes(attribute.String("hosts", queryHosts)))
	defer span.End()

	// Handle ANY (all hosts) case
	if types.IsAnySelector(queryHosts) {
		querierAnyable, ok := q.querier.(distributed.QuerierAnyable)
		if !ok {
			return nil, errQuerierTypeAllNotSupported
		}
		if hostList, err = querierAnyable.AllHosts(); err != nil {
			return nil, fmt.Errorf("failed to extract list of all hosts: %w", err)
		}

		return hostList, nil
	}

	// Default handling via resolver
	if hostList, err = resolver.Resolve(ctx, queryHosts); err != nil {
		err = fmt.Errorf("failed to resolve host list: %w", err)
		return nil, err
	}

	return hostList, nil
}

func (q *QueryRunner) checkSemaphore(stmt *query.Statement) (func(), error) {
	if q.sem == nil {
		return func() {}, nil
	}

	// Create a timeout context for waiting up to one keepalive interval
	semTimeout := DefaultSemTimeout
	if stmt.KeepAliveDuration > 0 {
		semTimeout = stmt.KeepAliveDuration
	}

	return q.sem.TryAddFor(semTimeout)
}

func (*QueryRunner) forwardKeepalives(ctx context.Context, keepaliveChan <-chan struct{}, send sse.Sender, keepaliveInterval time.Duration) {
	logger := logging.FromContext(ctx).With("keepalive_interval", keepaliveInterval)
	go func() {
		lastKeepalive := time.Now()
		for range keepaliveChan {
			// assess time since last keepalive emission and act accordingly
			if time.Since(lastKeepalive) > keepaliveInterval {
				lastKeepalive = time.Now()
				err := api.OnKeepalive(send)
				if err != nil {
					logger.With("error", err).Error("failed to handle keepalive event")
				}
			}
		}
	}()
}

// aggregateResults takes finished query workloads from the workloads channel, aggregates the result by merging the rows and summaries,
// and returns the final result. The `tracker` variable provides information about potential Run failures for individual hosts
func aggregateResults(ctx context.Context, stmt *query.Statement, queryResults <-chan *results.Result, send sse.Sender) (finalResult *results.Result) {
	ctx, span := tracing.Start(ctx, "aggregateResults")
	defer span.End()

	// aggregation
	finalResult = results.New()
	finalResult.Start()

	// tracker maps for meta info
	var (
		rowMap   = make(results.RowsMap)
		ifaceMap = make(map[string]struct{})
	)

	defer func() {
		finalizeResult(finalResult, stmt, rowMap, stmt.NumResults) // fully honors the limit
	}()

	for {
		select {
		case <-ctx.Done():
			return finalResult
		case qr, open := <-queryResults:
			if !open {
				return finalResult
			}
			aggregateSingleResult(ctx, qr, finalResult, stmt, ifaceMap, rowMap, send)
		}
	}
}

func aggregateSingleResult(ctx context.Context, qr, finalResult *results.Result, stmt *query.Statement, ifaceMap map[string]struct{}, rowMap results.RowsMap, send sse.Sender) {
	logger := logging.FromContext(ctx).With("hostname", qr.Hostname)
	if qr.Err() != nil {
		// unwrap the error if it's possible
		uerr := errors.Unwrap(qr.Err())
		if uerr == nil {
			uerr = qr.Err()
		}

		finalResult.HostsStatuses.SetErr(qr.Hostname, uerr)

		logger.Error(qr.Err())
		return
	}

	res := qr

	maps.Copy(finalResult.HostsStatuses, res.HostsStatuses)

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

	if send == nil {
		return
	}

	// for streaming, partial results must already include the current "final" state
	finalizeResult(finalResult, stmt, rowMap, maxLimitStreaming) // caps the limit if it exceeds maxLimitStreaming

	// if SSE callback is provided, run it
	err := api.OnResult(finalResult, send)
	if err != nil {
		logger.With("error", err).Error("failed to call results callback")
	}
}

func finalizeResult(res *results.Result, stmt *query.Statement, rowMap results.RowsMap, limitUpperBound uint64) {
	defer res.End()
	if len(rowMap) == 0 {
		return
	}

	// assign the rows to the result
	res.Rows = rowMap.ToRowsSorted(results.By(stmt.SortBy, stmt.Direction, stmt.SortAscending))

	limit := min(stmt.NumResults, limitUpperBound)

	// truncate by limit
	if limit < uint64(len(res.Rows)) {
		res.Rows = res.Rows[:limit]
	}
}
