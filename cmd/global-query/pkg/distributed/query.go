package distributed

import (
	"context"
	"fmt"
	"sync"

	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
)

type QueryRunner struct {
	resolver hosts.Resolver
	querier  Querier
}

func NewQueryRunner(resolver hosts.Resolver, querier Querier) *QueryRunner {
	return &QueryRunner{
		resolver: resolver,
		querier:  querier,
	}
}

func (q *QueryRunner) Run(ctx context.Context, args *query.Args) (*results.Result, error) {
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

	// query pipeline setup
	// sets up a fan-out, fan-in query processing pipeline
	numRunners := len(hostList)

	logger.With("runners", numRunners).Info("dispatching queries")

	finalResult := aggregateResults(ctx, stmt,
		runQueries(ctx, numRunners,
			prepareQueries(ctx, q.querier, hostList, &queryArgs),
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

// prepareQueries creates query workloads for all hosts in the host list and returns the channel it sends the
// workloads on
func prepareQueries(ctx context.Context, querier Querier, hostList hosts.Hosts, args *query.Args) <-chan *QueryWorkload {
	workloads := make(chan *QueryWorkload)

	go func(ctx context.Context) {
		logger := logging.WithContext(ctx)

		for _, host := range hostList {
			wl, err := querier.CreateQueryWorkload(ctx, host, args)
			if err != nil {
				logger.With("hostname", host).Errorf("failed to create workload: %v", err)
				continue
			}
			workloads <- wl
		}
		close(workloads)
	}(ctx)

	return workloads
}

// runQueries takes query workloads from the workloads channel, runs them, and returns a channel from which
// the results can be read
func runQueries(ctx context.Context, numRunners int, workloads <-chan *QueryWorkload) <-chan *queryResponse {
	out := make(chan *queryResponse, numRunners)

	wg := new(sync.WaitGroup)
	wg.Add(numRunners)
	for i := 0; i < numRunners; i++ {
		go func(ctx context.Context) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case wl, open := <-workloads:
					if !open {
						return
					}

					res, err := wl.Runner.Run(ctx, wl.Args)
					if err != nil {
						err = fmt.Errorf("failed to run query: %w", err)
					}

					qr := &queryResponse{
						host:   wl.Host,
						result: res,
						err:    err,
					}

					out <- qr
				}
			}
		}(ctx)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// aggregateResults takes finished query workloads from the workloads channel, aggregates the result by merging the rows and summaries,
// and returns the final result. The `tracker` variable provides information about potential Run failures for individual hosts
func aggregateResults(ctx context.Context, stmt *query.Statement, queryResults <-chan *queryResponse) (finalResult *results.Result) {
	// aggregation
	finalResult = results.New()
	finalResult.Start()

	var rowMap = make(results.RowsMap)

	// tracker maps for meta info
	var ifaceMap = make(map[string]struct{})

	logger := logging.WithContext(ctx)

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
			logger := logger.With("hostname", qr.host)
			if qr.err != nil {
				finalResult.HostsStatuses[qr.host] = results.Status{
					Code:    types.StatusError,
					Message: qr.err.Error(),
				}
				logger.Error(qr.err)
				continue
			}

			res := qr.result
			for host, status := range res.HostsStatuses {
				finalResult.HostsStatuses[host] = status
			}

			// merges the traffic data
			rowMap.MergeRows(res.Rows)

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
			finalResult.Summary.TimeFirst = res.Summary.TimeFirst
			finalResult.Summary.TimeLast = res.Summary.TimeLast
			finalResult.Summary.Totals = finalResult.Summary.Totals.Add(res.Summary.Totals)
			finalResult.Summary.Hits.Total = len(rowMap)
		}
	}
}

type QueryWorkload struct {
	Host string

	Runner query.Runner
	Args   *query.Args
}

type queryResponse struct {
	host   string
	result *results.Result
	err    error
}
