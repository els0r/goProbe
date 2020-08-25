package query

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/query/dns"
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
	Direction Direction `json:"direction"`

	// time selection
	First int64 `json:"from"`
	Last  int64 `json:"to"`

	// formatting
	Format        string    `json:"format"`
	NumResults    int       `json:"limit"`
	SortBy        SortOrder `json:"sort_by"`
	SortAscending bool      `json:"sort_ascending,omitempty"`
	Output        io.Writer `json:"-"`

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

	// query statistics
	Stats ExecutionStats `json:"query_stats"`

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

// ExecutionStats stores the statements execution statistics
type ExecutionStats struct {
	Start         time.Time     `json:"start"`
	Duration      time.Duration `json:"duration_ns"`
	HitsDisplayed int           `json:"hits_displayed"`
	Hits          int           `json:"hits"`
}

// String prints the statistics
func (e ExecutionStats) String() string {
	return fmt.Sprintf("{start: %s, query-duration: %s, hits: %d}",
		e.Start.Format(time.ANSIC),
		e.Duration,
		e.Hits,
	)
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
		str += fmt.Sprintf(", dns-resolution: %t, dns-timeout: %ds, dns-rows-resolved: %d",
			s.Resolve, s.ResolveTimeout, s.ResolveRows,
		)
	}
	// print statistics
	str += fmt.Sprintf(", stats: %s", s.Stats)
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
	err = jsoniter.NewEncoder(querylog).Encode(s)
}

// Execute runs the query with the provided parameters
func (s *Statement) Execute() error {

	var err error

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
			s.Stats.Duration = time.Now().Sub(s.Stats.Start)

			s.Err = &Error{err}
		}
		// log the query
		s.log()
	}()

	// Start timing
	s.Stats = ExecutionStats{
		Start: time.Now(),
	}

	// cross-check parameters
	if len(s.Ifaces) == 0 {
		return fmt.Errorf("no interfaces provided")
	}
	if s.Query == nil {
		return fmt.Errorf("query is not executable")
	}

	// create work managers
	workManagers := map[string]*goDB.DBWorkManager{} // map interfaces to workManagers
	for _, iface := range s.Ifaces {
		wm, nonempty, err := createWorkManager(s.DBPath, iface, s.First, s.Last, s.Query, numProcessingUnits)
		if err != nil {
			return err
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

			return err
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
			return s.noResults()
		default:
			return err
		}
	}

	/// DATA PRESENATION ///
	var mapEntries = make([]Entry, len(agg.aggregatedMap))
	var val goDB.Val
	count := 0

	for mapEntries[count].k, val = range agg.aggregatedMap {

		mapEntries[count].nBr = val.NBytesRcvd
		mapEntries[count].nPr = val.NPktsRcvd
		mapEntries[count].nBs = val.NBytesSent
		mapEntries[count].nPs = val.NPktsSent

		count++
	}

	// Now is a good time to release memory one last time for the final processing step
	agg.aggregatedMap = nil
	runtime.GC()
	debug.FreeOSMemory()

	// there is no need to sort influxdb datapoints
	if s.Format != "influxdb" {
		By(s.SortBy, s.Direction, s.SortAscending).Sort(mapEntries)
	}

	// Find map from ips to domains for reverse DNS
	var ips2domains map[string]string
	var resolveDuration time.Duration
	if s.Resolve && goDB.HasDNSAttributes(s.Query.Attributes) {
		var ips []string
		var sip, dip goDB.Attribute
		for _, attribute := range s.Query.Attributes {
			if attribute.Name() == "sip" {
				sip = attribute
			}
			if attribute.Name() == "dip" {
				dip = attribute
			}
		}

		for i, l := 0, len(mapEntries); i < l && i < s.ResolveRows; i++ {
			key := mapEntries[i].k
			if sip != nil {
				ips = append(ips, sip.ExtractStrings(&key)[0])
			}
			if dip != nil {
				ips = append(ips, dip.ExtractStrings(&key)[0])
			}
		}

		resolveStart := time.Now()
		ips2domains = dns.TimedReverseLookup(ips, s.ResolveTimeout)
		resolveDuration = time.Now().Sub(resolveStart)
	}

	// get the right printer
	var printer TablePrinter
	printer, err = s.NewTablePrinter(
		ips2domains,
		agg.totals,
		count,
	)
	if err != nil {
		return fmt.Errorf("failed to create printer: %s", err)
	}

	// stop timing everything related to the query and store the hits
	s.Stats.Duration = time.Now().Sub(s.Stats.Start)
	s.Stats.Hits = len(mapEntries)

	// fill the printer
	if s.NumResults < len(mapEntries) {
		mapEntries = mapEntries[:s.NumResults]
	}
	s.Stats.HitsDisplayed = len(mapEntries)
	for doneFilling := false; !doneFilling; {
		select {
		case err = <-memErrors:
			return fmt.Errorf("%w: %v", errorMemoryBreach, err)
		default:
			for _, entry := range mapEntries {
				printer.AddRow(entry)
			}
			doneFilling = true
		}
	}
	printer.Footer(s.Conditions, tSpanFirst, tSpanLast, s.Stats.Duration, resolveDuration)

	// print the data
	err = printer.Print()
	if err != nil {
		return err
	}

	return nil
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
