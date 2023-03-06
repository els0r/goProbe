package engine

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"time"

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
	query *goDB.Query
}

// NewQueryRunner creates a new query runner
func NewQueryRunner() *QueryRunner {
	return &QueryRunner{}
}

// Execute runs the query with the provided parameters
func (qr *QueryRunner) Run(ctx context.Context, stmt *query.Statement) (res []*results.Result, err error) {
	result := &results.Result{
		Status: types.StatusOK,
		Summary: results.Summary{
			Timings: results.Timings{
				// Start timing
				QueryStart: time.Now(),
			},
		},
	}
	// cross-check parameters
	if len(stmt.Ifaces) == 0 {
		return res, fmt.Errorf("no interfaces provided")
	}

	sort.Slice(stmt.Ifaces, func(i, j int) bool {
		return stmt.Ifaces[i] < stmt.Ifaces[j]
	})
	result.Summary.Interfaces = stmt.Ifaces

	// parse query
	var queryAttributes []types.Attribute

	queryAttributes, _, _, err = types.ParseQueryType(stmt.QueryType)
	if err != nil {
		return res, fmt.Errorf("failed to parse query type: %w", err)
	}

	// build condition tree to check if there is a syntax error before starting processing
	queryConditional, parseErr := node.ParseAndInstrument(stmt.Condition, stmt.DNSResolution.Timeout)
	if parseErr != nil {
		return res, fmt.Errorf("conditions parsing error: %w", parseErr)
	}

	qr.query = goDB.NewQuery(queryAttributes, queryConditional, stmt.HasAttrTime, stmt.HasAttrIface).LowMem(stmt.LowMem)
	if qr.query == nil {
		return res, fmt.Errorf("query is not executable")
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
	hostID := info.GetHostID()

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
			err = fmt.Errorf("%w: %v", errorMemoryBreach, err)
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

	// make sure execution stats and logging are taken care of
	defer func() {
		if err != nil {
			// get duration of execution even under error
			result.Summary.Timings.QueryDuration = time.Since(result.Summary.Timings.QueryStart)
			stmt.Err = err
		}
		// TODO: log the query
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
		wm, nonempty, err := createWorkManager(stmt.DBPath, iface, stmt.First, stmt.Last, qr.query, numProcessingUnits)
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

	result.Summary.TimeFirst = tSpanFirst
	result.Summary.TimeLast = tSpanLast

	// spawn reader processing units and make them work on the individual DB blocks
	// processing by interface is sequential, e.g. for multi-interface queries
	for _, workManager := range workManagers {
		workManager.ExecuteWorkerReadJobs(queryCtx, mapChan)
	}

	// we are done with all worker jobs
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
	result.Summary.Timings.QueryDuration = time.Since(result.Summary.Timings.QueryStart)
	result.Summary.Hits.Total = len(rs)

	if stmt.NumResults < len(rs) {
		rs = rs[:stmt.NumResults]
	}
	result.Summary.Hits.Displayed = len(rs)
	result.Rows = rs

	// set status and error message in case the query returned no results
	if len(result.Rows) == 0 {
		result.Status = types.StatusEmpty
		result.StatusMessage = results.ErrorNoResults.Error()
	}

	// assign the result
	res = []*results.Result{result}

	return res, nil
}

func createWorkManager(dbPath string, iface string, tfirst, tlast int64, query *goDB.Query, numProcessingUnits int) (workManager *goDB.DBWorkManager, nonempty bool, err error) {
	workManager, err = goDB.NewDBWorkManager(dbPath, iface, numProcessingUnits)
	if err != nil {
		return nil, false, fmt.Errorf("could not initialize query work manager for interface '%s': %s", iface, err)
	}
	nonempty, err = workManager.CreateWorkerJobs(tfirst, tlast, query)
	return
}
