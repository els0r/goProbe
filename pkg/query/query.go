package query

import (
	"context"
	"encoding/binary"
	"errors"
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
	Flat     bool   `json:"flat,omitempty"`     // serialize results as-is. Only makes sense in the json encoding case

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
)

// Error implements the error interface for query processing errors
func (i internalError) Error() string {
	var s string
	switch i {
	case errorNoResults:
		s = "query returned no results"
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
func (s *Statement) Execute() (result *results.Result, err error) {
	result = &results.Result{
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

	// start ticker to check memory consumption every second
	memErrors := make(chan error, 1)
	stopHeapWatch := watchHeap(s.MaxMemPct, memErrors)

	// make sure the memory ticker stops upon function return
	defer func() {

		// if a memory breach occurred, the memory monitor is shut
		// down already
		if errors.Is(err, errorMemoryBreach) {
			return
		}
		stopHeapWatch <- struct{}{}
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

	// Channel for handling of returned maps
	mapChan := make(chan map[goDB.ExtraKey]goDB.Val, 1024)
	aggregateChan := aggregate(mapChan)

	// spawn reader processing units and make them work on the individual DB blocks
	// processing by interface is sequential, e.g. for multi-interface queries
	for _, workManager := range workManagers {
		err = workManager.ExecuteWorkerReadJobs(mapChan, memErrors)
		if err != nil {

			// an error from the routine can only be of type memory error
			err = fmt.Errorf("%w: %v", errorMemoryBreach, err)

			// close the map channel. This will make sure that the aggregation routine
			// actually finishes
			close(mapChan)

			// empty the aggregateChan
			agg := <-aggregateChan

			// call the garbage collector
			agg.aggregatedMap = nil
			runtime.GC()
			debug.FreeOSMemory()

			return result, err
		}
	}

	// we are done with all worker jobs
	close(mapChan)

	// wait for the job to complete
	agg := <-aggregateChan
	err = agg.err
	if err != nil {
		switch err {
		case errorNoResults:
			return result, s.noResults()
		default:
			return result, err
		}
	}

	/// DATA PRESENTATION ///
	var sip, dip, dport, proto goDB.Attribute
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

	var rs = make(results.Rows, len(agg.aggregatedMap))
	count := 0

	for key, val := range agg.aggregatedMap {

		if key.Time != 0 {
			ts := time.Unix(key.Time, 0)
			rs[count].Labels.Timestamp = &ts
		}
		rs[count].Labels.Iface = key.Iface
		rs[count].Labels.HostID = key.HostID
		rs[count].Labels.Hostname = key.Hostname
		if sip != nil {
			rs[count].Attributes.SrcIP = goDB.RawIPToAddr(key.Sip[:])
		}
		if dip != nil {
			rs[count].Attributes.DstIP = goDB.RawIPToAddr(key.Dip[:])
		}
		if proto != nil {
			rs[count].Attributes.IPProto = key.Protocol
		}
		if dport != nil {
			rs[count].Attributes.DstPort = binary.LittleEndian.Uint16(key.Dport[:])
		}

		// assign counters
		rs[count].Counters.BytesReceived = val.NBytesRcvd
		rs[count].Counters.PacketsReceived = val.NPktsRcvd
		rs[count].Counters.BytesSent = val.NBytesSent
		rs[count].Counters.PacketsSent = val.NPktsSent

		count++
	}

	// Now is a good time to release memory one last time for the final processing step
	agg.aggregatedMap = nil
	runtime.GC()
	debug.FreeOSMemory()

	// Find map from ips to domains for reverse DNS
	var ips2domains map[string]string
	if s.Resolve && goDB.HasDNSAttributes(s.Query.Attributes) {
		var ips []string
		for i, l := 0, len(rs); i < l && i < s.ResolveRows; i++ {
			attr := rs[i].Attributes
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
	result.Summary.Totals = agg.totals

	// get the right printer
	var printer TablePrinter

	b := makeBasePrinter(
		s.Output,
		s.SortBy,
		s.HasAttrTime, s.HasAttrIface,
		s.Direction,
		s.Query.Attributes,
		ips2domains,
		agg.totals,
		strings.Join(s.Ifaces, ","),
	)

	switch s.Format {
	case "txt":
		printer = NewTextTablePrinter(b, count, s.ResolveTimeout)
	case "json":
		printer = NewJSONTablePrinter(b, s.QueryType, true)
	case "csv":
		printer = NewCSVTablePrinter(b)
	default:
		return result, fmt.Errorf("unknown output format %s", s.Format)
	}

	// sort the results
	results.By(s.SortBy, s.Direction, s.SortAscending).Sort(rs)

	// stop timing everything related to the query and store the hits
	result.Summary.Timings.QueryDuration = time.Since(result.Summary.Timings.QueryStart)
	result.Summary.Hits.Total = len(rs)

	// fill the printer
	if s.NumResults < len(rs) {
		rs = rs[:s.NumResults]
	}
	result.Summary.Hits.Displayed = len(rs)

	printCtx, printCancel := context.WithCancel(context.Background())
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
	err = printer.AddRows(printCtx, rs)
	if err != nil {
		if memErr != nil {
			return result, memErr
		}
		return result, err
	}
	printer.Footer(result)

	// print the data
	return result, printer.Print()
}

func createWorkManager(dbPath string, iface string, tfirst, tlast int64, query *goDB.Query, numProcessingUnits int) (workManager *goDB.DBWorkManager, nonempty bool, err error) {
	workManager, err = goDB.NewDBWorkManager(dbPath, iface, numProcessingUnits)
	if err != nil {
		return nil, false, fmt.Errorf("could not initialize query work manager for interface '%s': %s", iface, err)
	}
	nonempty, err = workManager.CreateWorkerJobs(tfirst, tlast, query)
	return
}

func (s *Statement) noResults() error {
	if s.External || s.Format == "json" {
		msg := ErrorMsgExternal{Status: "empty", Message: errorNoResults.Error()}
		return jsoniter.NewEncoder(s.Output).Encode(msg)
	}
	_, err := fmt.Fprintf(s.Output, "%s\n", errorNoResults.Error())
	return err
}
