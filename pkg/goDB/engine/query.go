package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/conditions/node"
	"github.com/els0r/goProbe/pkg/goDB/info"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/query/heap"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/els0r/goProbe/pkg/types/workload"
	"github.com/els0r/telemetry/tracing"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// QueryRunner implements the Runner interface to execute queries
// against the goDB flow database
type QueryRunner struct {
	query          *goDB.Query
	captureManager *capture.Manager
	dbPath         string

	keepAlive      time.Duration
	stats          *workload.Stats
	statsCallbacks workload.StatsFuncs
}

// RunnerOption allows to configure the query runner
type RunnerOption func(*QueryRunner)

// WithLiveData adds a capture manager which allows to query live data in addition to the data
// fetched from the DB
func WithLiveData(captureManager *capture.Manager) RunnerOption {
	return func(qr *QueryRunner) {
		qr.captureManager = captureManager
	}
}

// WithKeepAlive toggles keep-alive messages emitted by the runner. It's meant to signal to a
// calling process that a query is still being processed
func WithKeepAlive(interval time.Duration) RunnerOption {
	return func(qr *QueryRunner) {
		qr.keepAlive = interval
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

// NewQueryRunnerWithLiveData creates a new query runner that acts on both DB and live data
//
// DEPRECATED: use NewQueryrunner(dbPath, WithLiveData(captureManager)) instead
func NewQueryRunnerWithLiveData(dbPath string, captureManager *capture.Manager) *QueryRunner {
	return &QueryRunner{
		dbPath:         dbPath,
		captureManager: captureManager,
	}
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
func (dbLister DBInterfaceLister) GetInterfaces() ([]string, error) {
	return info.GetInterfaces(dbLister.dbPath)
}

const regExpSeparator = "/"

func isIFaceArgumentRegExp(iface string) bool {
	return strings.HasPrefix(iface, regExpSeparator) && strings.HasSuffix(iface, regExpSeparator) && len(iface) > 2
}

// Run implements the query.Runner interface
func (qr *QueryRunner) Run(ctx context.Context, args *query.Args) (res *results.Result, err error) {
	var argsStr string
	b, aerr := jsoniter.Marshal(args)
	if aerr == nil {
		argsStr = string(b)
	}

	ctx, span := tracing.Start(ctx, "(*engine.QueryRunner).Run", trace.WithAttributes(attribute.String("args", argsStr)))
	defer span.End()

	stmt, err := args.Prepare()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query statement: %w", err)
	}

	// get list of available interfaces in the local DB, filter based on given comma separated list or regexp,
	// reg exp is preferred
	var dbLister = NewDBInterfaceLister(qr.dbPath)

	if isIFaceArgumentRegExp(args.Ifaces) {
		iFaceRegexpArg := args.Ifaces
		iFacesRegExp := iFaceRegexpArg[1 : len(iFaceRegexpArg)-1]
		stmt.Ifaces, err = parseIfaceListWithRegex(dbLister, iFacesRegExp)
	} else {
		stmt.Ifaces, err = parseIfaceListWithCommaSeparatedString(dbLister, args.Ifaces)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to prepare query statement: %w", err)
	}

	return qr.RunStatement(ctx, stmt)
}

// RunStatement executes the prepared statement and generates the results
func (qr *QueryRunner) RunStatement(ctx context.Context, stmt *query.Statement) (res *results.Result, err error) {
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
	aggregateChan := qr.aggregate(ctx, mapChan, stmt.Ifaces, stmt.LowMem)

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

	// the covered time period is the union of all covered times
	tSpanFirst, tSpanLast := time.Now().AddDate(100, 0, 0), time.Time{} // a hundred years in the future, the beginning of time
	for _, workManager := range workManagers {
		t0, t1 := workManager.GetCoveredTimeInterval()
		if t0.Before(tSpanFirst) {
			tSpanFirst = t0
		}
		if tSpanLast.Before(t1) {
			tSpanLast = t1
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

		// add statistics to final result
		result.Summary.Stats.Add(aggMap.Stats)

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

func (qr *QueryRunner) runLiveQuery(ctx context.Context, mapChan chan hashmap.AggFlowMapWithMetadata, stmt *query.Statement) (wg *sync.WaitGroup) {
	wg = new(sync.WaitGroup)

	if !stmt.Live {
		return
	}

	wg.Add(1)
	go func() {
		qr.captureManager.GetFlowMaps(ctx, goDB.QueryFilter(qr.query), mapChan, stmt.Ifaces...)
		wg.Done()
	}()

	return
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

	allIfaces, err := lister.GetInterfaces()
	if err != nil {
		return nil, err
	}

	selectedValidIfaces, negationFilters, err := types.ValidateAndSeparateFilters(ifaceList)
	if err != nil {
		return nil, err
	}

	// add interfaces
	var result []string
	for _, iface := range selectedValidIfaces {
		if types.IsAnySelector(iface) {
			result = allIfaces
			break
		} else if slices.Contains(allIfaces, iface) {
			result = append(result, iface)
		}

	}

	// remove interfaces
	for _, notIface := range negationFilters {
		if slices.Contains(result, notIface) {
			for i, v := range result {
				if v == notIface {
					// Remove the element by slicing
					result = append(result[:i], result[i+1:]...)
				}
			}
		}
	}
	return result, nil
}

func parseIfaceListWithRegex(lister types.InterfaceLister, ifaceRegExp string) ([]string, error) {

	ifaces, err := lister.GetInterfaces()
	if err != nil {
		return nil, err
	}
	regexp, reErr := regexp.Compile(ifaceRegExp)
	if reErr != nil {
		return nil, reErr
	}

	var filteredIfaces []string
	for _, iface := range ifaces {
		if regexp.MatchString(iface) {
			filteredIfaces = append(filteredIfaces, iface)
		}
	}
	return filteredIfaces, nil
}
