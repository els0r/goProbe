package query

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/query/dns"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	jsoniter "github.com/json-iterator/go"
)

var numProcessingUnits = runtime.NumCPU()

// Statement bundles all relevant options for running a query and displaying its result
type Statement struct {
	// Ifaces holds hte list of all interfaces that should be queried
	Ifaces []string `json:"ifaces"`

	// Query holds attributes and conditions for execution
	Query        *goDB.Query `json:"-"`
	HasAttrIface bool        `json:"-"`
	HasAttrTime  bool        `json:"-"`

	// needed for feedback to user
	Conditions string `json:"condition,omitempty"`
	QueryType  string `json:"query_type"`

	// which direction is added
	Direction types.Direction `json:"direction"`

	// time selection
	First int64 `json:"from"`
	Last  int64 `json:"to"`

	// formatting
	Format        string            `json:"format"`
	NumResults    int               `json:"limit"`
	SortBy        results.SortOrder `json:"sort_by"`
	SortAscending bool              `json:"sort_ascending,omitempty"`
	Output        io.Writer         `json:"-"`

	// parameters for external calls
	External bool   `json:"external,omitempty"` // for error messages
	Caller   string `json:"caller,omitempty"`   // who called the query

	// resolution parameters (probably part of table printer)
	Resolve        bool          `json:"dns_resolution,omitempty"`
	ResolveTimeout time.Duration `json:"dns_timeout,omitempty"`
	ResolveRows    int           `json:"-"`

	// file system
	DBPath    string `json:"db"`
	MaxMemPct int    `json:"-"`

	// error during execution
	Err *Error `json:"error,omitempty"`
}

// Error encloses an error encountered during processing
type Error struct{ err error }

func (q *Error) Error() string { return q.err.Error() }

type internalError int

// enumeration of processing errors
const (
	errorNoResults internalError = iota + 1
	errorMemoryBreach
	errorInternalProcessing
	errorMismatchingHosts
)

// Error implements the error interface for query processing errors
func (i internalError) Error() string {
	var s string
	switch i {
	case errorMemoryBreach:
		s = "memory limit exceeded"
	case errorInternalProcessing:
		s = "internal error during query processing"
	}
	return s
}

// MarshalJSON implements the Marshaler interface for human-readable error logging
func (q *Error) MarshalJSON() ([]byte, error) {
	return jsoniter.Marshal(q.Error())
}

// String prints the executable statement in human-readable form
func (s *Statement) String() string {
	str := fmt.Sprintf("{type: %s, ifaces: %s",
		s.QueryType,
		s.Ifaces,
	)
	if s.Conditions != "" {
		str += fmt.Sprintf(", condition: %s", s.Conditions)
	}
	tFrom, tTo := time.Unix(s.First, 0), time.Unix(s.Last, 0)
	str += fmt.Sprintf(", db: %s, limit: %d, from: %s, to: %s",
		s.DBPath,
		s.NumResults,
		tFrom.Format(time.ANSIC),
		tTo.Format(time.ANSIC),
	)
	if s.Resolve {
		str += fmt.Sprintf(", dns-resolution: %t", s.Resolve)
	}
	str += "}"
	return str
}

// log writes a json marshaled query statement to disk
func (s *Statement) log() {

	// open the file in append mode
	querylog, err := os.OpenFile(filepath.Join(s.DBPath, goDB.QueryLogFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer querylog.Close()

	// opportunistically write statement to disk
	_ = jsoniter.NewEncoder(querylog).Encode(s)
}

// Execute runs the query with the provided parameters
func (s *Statement) Execute(ctx context.Context) (result *results.Result, err error) {
	result = &results.Result{
		Status: types.StatusOK,
		Summary: results.Summary{
			Timings: results.Timings{
				// Start timing
				QueryStart: time.Now(),
			},
		},
	}
	sort.Slice(s.Ifaces, func(i, j int) bool {
		return s.Ifaces[i] < s.Ifaces[j]
	})
	result.Summary.Interfaces = s.Ifaces

	// cross-check parameters
	if len(s.Ifaces) == 0 {
		return result, fmt.Errorf("no interfaces provided")
	}
	if s.Query == nil {
		return result, fmt.Errorf("query is not executable")
	}
	result.Query = results.Query{
		Attributes: s.Query.AttributesToString(),
	}
	if s.Query.Conditional != nil {
		result.Query.Condition = s.Query.Conditional.String()
	}

	// start ticker to check memory consumption every second
	heapWatchCtx, cancelHeapWatch := context.WithCancel(ctx)
	defer cancelHeapWatch()

	memErrors := watchHeap(heapWatchCtx, s.MaxMemPct)

	queryCtx, cancelQuery := context.WithCancel(ctx)
	defer cancelQuery()

	// Channel for handling of returned maps
	mapChan := make(chan hashmap.AggFlowMapWithMetadata, 1024)
	aggregateChan := aggregate(mapChan, s.Query.IsLowMem())

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
			agg.aggregatedMap.Map = nil
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
			s.Err = &Error{err}
		}
		// log the query
		s.log()
	}()

	// cross-check parameters
	if len(s.Ifaces) == 0 {
		return result, fmt.Errorf("no interfaces provided")
	}
	if s.Query == nil {
		return result, fmt.Errorf("query is not executable")
	}
	result.Query = results.Query{
		Attributes: s.Query.AttributesToString(),
	}
	if s.Query.Conditional != nil {
		result.Query.Condition = s.Query.Conditional.String()
	}

	// create work managers
	workManagers := map[string]*goDB.DBWorkManager{} // map interfaces to workManagers
	for _, iface := range s.Ifaces {
		wm, nonempty, err := createWorkManager(s.DBPath, iface, s.First, s.Last, s.Query, numProcessingUnits)
		if err != nil {
			return result, err
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
		return result, err
	}

	// check aggregation for errors
	if agg.err != nil {
		return result, agg.err
	}

	/// RESULTS PREPARATION ///
	var sip, dip, dport, proto types.Attribute
	for _, attribute := range s.Query.Attributes {
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

	var rs = make(results.Rows, agg.aggregatedMap.Len())
	count := 0

	for i := agg.aggregatedMap.Iter(); i.Next(); {

		key := types.ExtendedKey(i.Key())
		val := i.Val()

		if ts, hasTS := key.AttrTime(); hasTS {
			rs[count].Labels.Timestamp = time.Unix(ts, 0)
		}
		rs[count].Labels.Iface, _ = key.AttrIface()
		rs[count].Labels.HostID = agg.aggregatedMap.HostID
		rs[count].Labels.Hostname = agg.aggregatedMap.Hostname
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
	if s.Query.IsLowMem() {
		agg.aggregatedMap.Map.Clear()
	} else {
		agg.aggregatedMap.Map.ClearFast()
		agg.aggregatedMap.Map = nil
	}
	runtime.GC()

	result.Summary.Totals = agg.totals

	// sort the results
	results.By(s.SortBy, s.Direction, s.SortAscending).Sort(rs)

	// stop timing everything related to the query and store the hits
	result.Summary.Timings.QueryDuration = time.Since(result.Summary.Timings.QueryStart)
	result.Summary.Hits.Total = len(rs)

	if s.NumResults < len(rs) {
		rs = rs[:s.NumResults]
	}
	result.Summary.Hits.Displayed = len(rs)
	result.Rows = rs

	return result, nil
}

func (s *Statement) Print(ctx context.Context, result *results.Result) error {
	var sip, dip types.Attribute

	var hasDNSattributes bool
	for _, attribute := range s.Query.Attributes {
		switch attribute.Name() {
		case "sip":
			sip = attribute
			hasDNSattributes = true
		case "dip":
			dip = attribute
			hasDNSattributes = true
		}
	}

	// Find map from ips to domains for reverse DNS
	var ips2domains map[string]string
	if s.Resolve && hasDNSattributes {
		var ips []string
		for i, l := 0, len(result.Rows); i < l && i < s.ResolveRows; i++ {
			attr := result.Rows[i].Attributes
			if sip != nil {
				ips = append(ips, attr.SrcIP.String())
			}
			if dip != nil {
				ips = append(ips, attr.DstIP.String())
			}
		}

		resolveStart := time.Now()
		ips2domains = dns.TimedReverseLookup(ips, s.ResolveTimeout)
		result.Summary.Timings.ResolutionDuration = time.Since(resolveStart)
	}

	// get the right printer
	printer, err := results.NewTablePrinter(
		s.Output,
		s.Format,
		s.SortBy,
		s.HasAttrTime, s.HasAttrIface,
		s.Direction,
		s.Query.Attributes,
		ips2domains,
		result.Summary.Totals,
		result.Summary.Hits.Total,
		s.ResolveTimeout,
		s.QueryType,
		strings.Join(s.Ifaces, ","),
	)
	if err != nil {
		return err
	}

	// start ticker to check memory consumption every second
	heapWatchCtx, cancelHeapWatch := context.WithCancel(ctx)
	defer cancelHeapWatch()

	memErrors := watchHeap(heapWatchCtx, s.MaxMemPct)

	printCtx, printCancel := context.WithCancel(ctx)
	defer printCancel()

	var memErr error
	go func() {
		select {
		case memErr = <-memErrors:
			memErr = fmt.Errorf("%w: %v", errorMemoryBreach, err)
			printCancel()
			return
		case <-printCtx.Done():
			return
		}
	}()
	err = printer.AddRows(printCtx, result.Rows)
	if err != nil {
		if memErr != nil {
			return memErr
		}
		return err
	}
	printer.Footer(result)

	return printer.Print()
}

func createWorkManager(dbPath string, iface string, tfirst, tlast int64, query *goDB.Query, numProcessingUnits int) (workManager *goDB.DBWorkManager, nonempty bool, err error) {
	workManager, err = goDB.NewDBWorkManager(dbPath, iface, numProcessingUnits)
	if err != nil {
		return nil, false, fmt.Errorf("could not initialize query work manager for interface '%s': %s", iface, err)
	}
	nonempty, err = workManager.CreateWorkerJobs(tfirst, tlast, query)
	return
}
