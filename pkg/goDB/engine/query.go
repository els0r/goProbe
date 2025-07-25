package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"time"

	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/els0r/goProbe/v4/pkg/goDB"
	"github.com/els0r/goProbe/v4/pkg/goDB/conditions/node"
	"github.com/els0r/goProbe/v4/pkg/goDB/info"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/query/heap"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/goProbe/v4/pkg/types/hashmap"
	"github.com/els0r/goProbe/v4/pkg/types/workload"
	"github.com/els0r/telemetry/tracing"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	// DefaultSemTimeout is the default / fallback amount of time to wait for acquisition
	// of a semaphore when performing concurrent queries
	DefaultSemTimeout = time.Second
)

// RunnerOption allows to configure the query runner
type RunnerOption func(*QueryRunner)

// WithMaxConcurrent sets a maximum number of concurrent running queries
func WithMaxConcurrent(sem chan struct{}) RunnerOption {
	return func(qr *QueryRunner) {
		qr.sem = sem
	}
}

// WithStatsCallbacks configures the DBWorkManager to emit feedback on query processing. They will be called in
// the order they are provided
func WithStatsCallbacks(callbacks ...workload.StatsFunc) RunnerOption {
	return func(qr *QueryRunner) {
		qr.statsCallbacks = callbacks
	}
}

// NewQueryRunner creates a new query runner
func NewQueryRunner(dbPath string, opts ...RunnerOption) *QueryRunner {
	qr := &QueryRunner{
		dbPath: dbPath,
	}

	for _, opt := range opts {
		opt(qr)
	}
	return qr
}

// DBLister lists network interfaces from DB.
type DBInterfaceLister struct {
	dbPath string
}

// Constructor for DBInterfaceLister
func NewDBInterfaceLister(dbPath string) *DBInterfaceLister {
	return &DBInterfaceLister{dbPath: dbPath}
}

// Implementation of the interface function that uses existing functionality.
func (dbLister DBInterfaceLister) ListInterfaces() ([]string, error) {
	return info.GetInterfaces(dbLister.dbPath)
}

// Run implements the query.Runner interface
func (qr *QueryRunner) Run(ctx context.Context, args *query.Args) (res *results.Result, err error) {
	ctx, span := tracing.Start(ctx, "(*engine.QueryRunner).Run")
	defer span.End()

	return qr.run(ctx, args, nil)
}

// RunStreaming implements the api.SSEQueryRunner interface
func (qr *QueryRunner) RunStreaming(ctx context.Context, args *query.Args, send sse.Sender) (res *results.Result, err error) {
	ctx, span := tracing.Start(ctx, "(*engine.QueryRunner).Run")
	defer span.End()

	return qr.run(ctx, args, send)
}

func (qr *QueryRunner) run(ctx context.Context, args *query.Args, send sse.Sender) (res *results.Result, err error) {
	var argsStr string
	b, aerr := jsoniter.Marshal(args)
	if aerr == nil {
		argsStr = string(b)
	}

	ctx, span := tracing.Start(ctx, "(*engine.QueryRunner).run", trace.WithAttributes(attribute.String("args", argsStr)))
	defer span.End()

	stmt, err := args.Prepare()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query statement: %w", err)
	}

	smeDone, err := qr.checkSemaphore(stmt)
	if err != nil {
		return &results.Result{
			Status: results.Status{
				Code:    types.StatusTooManyRequests,
				Message: "too many concurrent requests",
			},
		}, nil
	}
	defer smeDone()

	// get list of available interfaces in the local DB, filter based on given comma separated list or regexp,
	// reg exp is preferred
	var dbLister = NewDBInterfaceLister(qr.dbPath)

	if types.IsIfaceArgumentRegExp(args.Ifaces) {
		stmt.Ifaces, err = parseIfaceListWithRegex(dbLister, args.Ifaces)
	} else {
		stmt.Ifaces, err = parseIfaceListWithCommaSeparatedString(dbLister, args.Ifaces)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to prepare query statement: %w", err)
	}

	return qr.RunStatement(ctx, stmt, send)
}

// RunStatement executes the prepared statement and generates the results
func (qr *QueryRunner) RunStatement(ctx context.Context, stmt *query.Statement, send sse.Sender) (res *results.Result, err error) {
	result := results.New()
	result.Start()
	defer result.End()

	// cross-check parameters
	if len(stmt.Ifaces) == 0 {
		return res, errors.New("no interfaces provided")
	}

	sort.Slice(stmt.Ifaces, func(i, j int) bool {
		return stmt.Ifaces[i] < stmt.Ifaces[j]
	})
	result.Summary.Interfaces = stmt.Ifaces
	if stmt.KeepAliveDuration > 0 {
		qr.keepAlive = stmt.KeepAliveDuration
	}

	// parse query
	queryAttributes, _, err := types.ParseQueryType(stmt.QueryType)
	if err != nil {
		return res, fmt.Errorf("failed to parse query type: %w", err)
	}

	// build condition tree to check if there is a syntax error before starting processing
	queryConditional, valFilterNode, parseErr := node.ParseAndInstrument(stmt.Condition, stmt.DNSResolution.Timeout)
	if parseErr != nil {
		return res, fmt.Errorf("conditions parsing error: %w", parseErr)
	}

	qr.query = goDB.NewQuery(queryAttributes, queryConditional, stmt.LabelSelector).LowMem(stmt.LowMem)
	if qr.query == nil {
		return res, errors.New("query is not executable")
	}

	result.Query = results.Query{
		Attributes: qr.query.AttributesToString(),
	}
	result.Query.Condition = node.QueryConditionalString(qr.query.Conditional, valFilterNode)

	// get hostname and host ID if available
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get system hostname: %w", err)
	}
	hostID := info.GetHostID(qr.dbPath)
	result.Hostname = hostname

	// assign the hostname to the list of hosts handled in this query. Here, the only one
	defer func() {
		result.HostsStatuses[hostname] = result.Status
	}()

	// start ticker to check memory consumption every second
	heapWatchCtx, cancelHeapWatch := context.WithCancel(ctx)
	defer cancelHeapWatch()

	memErrors := heap.Watch(heapWatchCtx, stmt.MaxMemPct)

	queryCtx, cancelQuery := context.WithCancel(ctx)
	defer cancelQuery()

	// Channel for handling of returned maps
	mapChan := make(chan hashmap.AggFlowMapWithMetadata, 1024)
	aggregateChan := qr.aggregate(ctx, mapChan, send, stmt.Ifaces, stmt.LowMem)

	go func() {
		select {
		case err = <-memErrors:
			err = fmt.Errorf("%w: %w", errorMemoryBreach, err)
			cancelQuery()

			// close the map channel. This will make sure that the aggregation routine
			// actually finishes
			close(mapChan)

			// empty the aggregateChan
			agg := <-aggregateChan

			// call the garbage collector
			agg.aggregatedMaps.ClearFast()
			runtime.GC()
			debug.FreeOSMemory()

			return
		case <-queryCtx.Done():
			return
		}
	}()

	var opts = []goDB.WorkManagerOption{}

	// create work managers
	workManagers := map[string]*goDB.DBWorkManager{} // map interfaces to workManagers
	for _, iface := range stmt.Ifaces {
		wm, nonempty, err := createWorkManager(qr.dbPath, iface, stmt.First, stmt.Last, qr.query, numProcessingUnits, opts...)
		if err != nil {
			return res, err
		}
		// Only add work managers that have work to do.
		if nonempty {
			workManagers[iface] = wm
		}
	}

	// the covered time period is the union of all covered times (initialize to query time period in case no work managers are created
	tSpanFirst, tSpanLast := time.Unix(stmt.First, 0), time.Unix(stmt.Last, 0)
	if len(workManagers) > 0 {
		tSpanFirst, tSpanLast = time.Now().AddDate(100, 0, 0), time.Time{} // a hundred years in the future, the beginning of time
		for _, workManager := range workManagers {
			t0, t1 := workManager.GetCoveredTimeInterval()
			if t0.Before(tSpanFirst) {
				tSpanFirst = t0
			}
			if tSpanLast.Before(t1) {
				tSpanLast = t1
			}
		}
	}

	// Check if there actually was data available from disk (or a live query was performed)
	if len(workManagers) > 0 || stmt.Live {
		result.Summary.DataAvailable = true
	}

	result.Summary.First = tSpanFirst
	result.Summary.Last = tSpanLast

	// If enabled, run a live query in the background / parallel to the DB query and put the results on the same output channel
	liveQueryWG := qr.runLiveQuery(queryCtx, mapChan, stmt)

	// spawn reader processing units and make them work on the individual DB blocks
	// processing by interface is sequential, e.g. for multi-interface queries
	for _, workManager := range workManagers {
		workManager.ExecuteWorkerReadJobs(queryCtx, mapChan)
	}

	// In case a live query is being performed in the background, ensure it is done
	liveQueryWG.Wait()

	// We are done with all worker jobs, close the ouput / result channel
	close(mapChan)

	// wait for the job to complete, then call a garbage collection
	agg := <-aggregateChan
	for _, workManager := range workManagers {
		workManager.Close()
		workManager = nil
	}
	runtime.GC()

	// first inspect if err is set due to problems not related to aggregation
	if err != nil {
		return res, err
	}

	// check aggregation for errors
	if agg.err != nil {
		return res, agg.err
	}

	/// RESULTS PREPARATION ///
	var sip, dip, dport, proto types.Attribute
	for _, attribute := range qr.query.Attributes {
		switch attribute.Name() {
		case types.SIPName:
			sip = attribute
		case types.DIPName:
			dip = attribute
		case types.DportName:
			dport = attribute
		case types.ProtoName:
			proto = attribute
		}
	}

	var rs = make(results.Rows, agg.aggregatedMaps.Len())
	count := 0

	var metaIterOption hashmap.MetaIterOption
	if valFilterNode != nil && valFilterNode.ValFilter != nil {
		metaIterOption = hashmap.WithFilter(valFilterNode.ValFilter)
	}
	var totals hashmap.Val
	for iface, aggMap := range agg.aggregatedMaps {
		var i = aggMap.Iter()
		if metaIterOption != nil {
			i = aggMap.Iter(metaIterOption)
		}
		for i.Next() {

			key := types.ExtendedKey(i.Key())
			val := i.Val()
			totals.Add(val)
			if ts, hasTS := key.AttrTime(); hasTS {
				rs[count].Labels.Timestamp = time.Unix(ts, 0)
			}
			rs[count].Labels.Iface = iface

			// the host ID and hostname are statically assigned since a goDB is inherently limited to the
			// system it runs on. The two parameters never change during query execution
			rs[count].Labels.HostID = hostID
			rs[count].Labels.Hostname = hostname

			if sip != nil {
				rs[count].Attributes.SrcIP = types.RawIPToAddr(key.Key().GetSIP())
			}
			if dip != nil {
				rs[count].Attributes.DstIP = types.RawIPToAddr(key.Key().GetDIP())
			}
			if proto != nil {
				rs[count].Attributes.IPProto = key.Key().GetProto()
			}
			if dport != nil {
				rs[count].Attributes.DstPort = types.PortToUint16(key.Key().GetDport())
			}

			// assign / update counters
			rs[count].Counters.Add(val)
			count++
		}

		// add statistics to final result and trigger keepalive (if required)
		result.Summary.Stats.Add(aggMap.Stats)
		select {
		case <-queryCtx.Done():
			return
		default:
			qr.query.UpdateKeepalive()
		}

		// Now is a good time to release memory one last time for the final processing step
		if qr.query.IsLowMem() {
			aggMap.Clear()
		} else {
			aggMap.ClearFast()
		}
		runtime.GC()
	}

	// Ensure that potentially unused pre-allocated rows are dropped
	rs = rs[:count]

	result.Summary.Totals = totals

	// sort the results
	results.By(stmt.SortBy, stmt.Direction, stmt.SortAscending).Sort(rs)

	// stop timing everything related to the query and store the hits
	result.Summary.Hits.Total = count

	// due to filtering, might display less than min(stmt.NumResults, len(rs))
	// result rows
	nDisplay := stmt.NumResults
	if uint64(count) < stmt.NumResults {
		nDisplay = uint64(count)
	}
	if nDisplay < uint64(len(rs)) {
		rs = rs[:nDisplay]
	}
	result.Summary.Hits.Displayed = len(rs)
	result.Rows = rs
	return result, nil
}

func (qr *QueryRunner) checkSemaphore(stmt *query.Statement) (func(), error) {
	if qr.sem == nil {
		return func() {}, nil
	}

	// Create a timeout context for waiting up to one keepalive interval
	semTimeout := DefaultSemTimeout
	if stmt.KeepAliveDuration > 0 {
		semTimeout = stmt.KeepAliveDuration
	}

	return qr.sem.TryAddFor(semTimeout)
}

func createWorkManager(dbPath string, iface string, tfirst, tlast int64, query *goDB.Query, numProcessingUnits int, opts ...goDB.WorkManagerOption) (workManager *goDB.DBWorkManager, nonempty bool, err error) {
	workManager, err = goDB.NewDBWorkManager(query, dbPath, iface, numProcessingUnits, opts...)
	if err != nil {
		return nil, false, fmt.Errorf("could not initialize query work manager for interface '%s': %w", iface, err)
	}
	nonempty, err = workManager.CreateWorkerJobs(tfirst, tlast)
	return
}

func parseIfaceListWithCommaSeparatedString(lister types.InterfaceLister, ifaceList string) ([]string, error) {
	if ifaceList == "" {
		return nil, errors.New("no interface(s) specified")
	}

	allIfaces, err := lister.ListInterfaces()
	if err != nil {
		return nil, err
	}

	selectedValidIfaces, negationFilters, err := types.ValidateAndSeparateFilters(ifaceList)
	if err != nil {
		return nil, err
	}

	// add interfaces
	var resultingIfaces []string
	for _, iface := range selectedValidIfaces {
		if types.IsAnySelector(iface) {
			resultingIfaces = allIfaces
			break
		} else if slices.Contains(allIfaces, iface) {
			resultingIfaces = append(resultingIfaces, iface)
		}

	}

	// remove interfaces
	for _, notIface := range negationFilters {
		for i, v := range resultingIfaces {
			if v == notIface {
				// Remove the element by slicing
				resultingIfaces = append(resultingIfaces[:i], resultingIfaces[i+1:]...)
			}
		}
	}
	return resultingIfaces, nil
}

func parseIfaceListWithRegex(lister types.InterfaceLister, ifaceRegExp string) ([]string, error) {

	regexp, reErr := types.ValidateAndExtractRegExp(ifaceRegExp)
	if reErr != nil {
		return nil, reErr
	}

	ifaces, err := lister.ListInterfaces()
	if err != nil {
		return nil, err
	}

	var filteredIfaces []string
	for _, iface := range ifaces {
		if regexp.MatchString(iface) {
			filteredIfaces = append(filteredIfaces, iface)
		}
	}
	return filteredIfaces, nil
}
