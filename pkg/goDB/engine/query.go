package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
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
)

// QueryRunner implements the Runner interface to execute queries
// against the goDB flow database
type QueryRunner struct {
	query          *goDB.Query
	captureManager *capture.Manager
	dbPath         string
}

// NewQueryRunner creates a new query runner
func NewQueryRunner(dbPath string) *QueryRunner {
	return &QueryRunner{
		dbPath: dbPath,
	}
}

// NewQueryRunnerWithLiveData creates a new query runner that acts on both DB and live data
func NewQueryRunnerWithLiveData(dbPath string, captureManager *capture.Manager) *QueryRunner {
	return &QueryRunner{
		dbPath:         dbPath,
		captureManager: captureManager,
	}
}

// Run implements the query.Runner interface
func (qr *QueryRunner) Run(ctx context.Context, args *query.Args) (res *results.Result, err error) {
	stmt, err := args.Prepare()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query statement: %w", err)
	}

	// get list of available interfaces in the local DB
	stmt.Ifaces, err = parseIfaceList(qr.dbPath, args.Ifaces)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query statement: %w", err)
	}

	return qr.RunStatement(ctx, stmt)
}

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
	queryConditional, parseErr := node.ParseAndInstrument(stmt.Condition, stmt.DNSResolution.Timeout)
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
	if qr.query.Conditional != nil {
		result.Query.Condition = qr.query.Conditional.String()
	}

	// get hostname and host ID if available
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get system hostname: %w", err)
	}
	hostID := info.GetHostID(qr.dbPath)

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
	aggregateChan := aggregate(mapChan, stmt.Ifaces, stmt.LowMem)

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

	result.Query = results.Query{
		Attributes: qr.query.AttributesToString(),
	}
	if qr.query.Conditional != nil {
		result.Query.Condition = qr.query.Conditional.String()
	}

	// create work managers
	workManagers := map[string]*goDB.DBWorkManager{} // map interfaces to workManagers
	for _, iface := range stmt.Ifaces {
		wm, nonempty, err := createWorkManager(qr.dbPath, iface, stmt.First, stmt.Last, qr.query, numProcessingUnits)
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

	result.Summary.First = tSpanFirst
	result.Summary.Last = tSpanLast

	// If enabled, run a live query in the background / parallel to the DB query and put the results on the same output channel
	liveQueryWG := qr.runLiveQuery(ctx, mapChan, stmt)

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
		case "sip":
			sip = attribute
		case "dip":
			dip = attribute
		case "dport":
			dport = attribute
		case "proto":
			proto = attribute
		}
	}

	var rs = make(results.Rows, agg.aggregatedMaps.Len())
	count := 0

	for iface, aggMap := range agg.aggregatedMaps {
		for i := aggMap.Iter(); i.Next(); {

			key := types.ExtendedKey(i.Key())
			val := i.Val()

			if ts, hasTS := key.AttrTime(); hasTS {
				rs[count].Labels.Timestamp = time.Unix(ts, 0)
			}
			rs[count].Labels.Iface = iface

			// the host ID and hostname are statically assigned since a goDB is inherently limited to the
			// system it runs on. The two parameters never change during query execution
			rs[count].Labels.HostID = hostID
			rs[count].Labels.Hostname = hostname

			if sip != nil {
				rs[count].Attributes.SrcIP = types.RawIPToAddr(key.Key().GetSip())
			}
			if dip != nil {
				rs[count].Attributes.DstIP = types.RawIPToAddr(key.Key().GetDip())
			}
			if proto != nil {
				rs[count].Attributes.IPProto = key.Key().GetProto()
			}
			if dport != nil {
				rs[count].Attributes.DstPort = types.PortToUint16(key.Key().GetDport())
			}

			// assign / update counters
			rs[count].Counters = rs[count].Counters.Add(val)
			count++
		}

		// Now is a good time to release memory one last time for the final processing step
		if qr.query.IsLowMem() {
			aggMap.Clear()
		} else {
			aggMap.ClearFast()
		}
		runtime.GC()
	}

	result.Summary.Totals = agg.totals

	// sort the results
	results.By(stmt.SortBy, stmt.Direction, stmt.SortAscending).Sort(rs)

	// stop timing everything related to the query and store the hits
	result.Summary.Hits.Total = len(rs)

	if stmt.NumResults < len(rs) {
		rs = rs[:stmt.NumResults]
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

func createWorkManager(dbPath string, iface string, tfirst, tlast int64, query *goDB.Query, numProcessingUnits int) (workManager *goDB.DBWorkManager, nonempty bool, err error) {
	workManager, err = goDB.NewDBWorkManager(query, dbPath, iface, numProcessingUnits)
	if err != nil {
		return nil, false, fmt.Errorf("could not initialize query work manager for interface '%s': %w", iface, err)
	}
	nonempty, err = workManager.CreateWorkerJobs(tfirst, tlast)
	return
}

func parseIfaceList(dbPath string, ifacelist string) (ifaces []string, err error) {
	if ifacelist == "" {
		return nil, errors.New("no interface(s) specified")
	}

	if strings.ToLower(ifacelist) == "any" {
		ifaces, err = info.GetInterfaces(dbPath)
		if err != nil {
			return nil, err
		}
	} else {
		if ifaces, err = parseIfaceNames(ifacelist); err != nil {
			return
		}
	}
	return
}

var ifaceNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9\.:_-]{1,15}$`)

func validateIfaceName(iface string) error {
	if iface == "" {
		return errors.New("interface list contains empty interface name")
	}

	if !ifaceNameRegexp.MatchString(iface) {
		return fmt.Errorf("interface name `%s` is invalid", iface)
	}

	return nil
}

func parseIfaceNames(ifacelist string) (ifaces []string, err error) {
	ifaces = strings.Split(ifacelist, ",")
	for _, iface := range ifaces {
		if err = validateIfaceName(iface); err != nil {
			return
		}
	}
	return
}
