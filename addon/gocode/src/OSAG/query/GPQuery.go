/////////////////////////////////////////////////////////////////////////////////
//
// GPQuery.go
//
// nquery replacement
//
// Written by Fabian Kohn   fko@open.ch and
//            Lennart Elsen lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, September 2015
// Copyright (c) 2014 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"
	//    "runtime/pprof"

	"OSAG/goDB"
	"OSAG/version"
)

const (
	// Variables for manual garbage collection calls
	GOGCINTERVAL     = 5 * time.Second
	GOGCLIMIT        = 6291456 // Limit for GC call, in bytes
	MEMCHECKINTERVAL = 1 * time.Second
	// Default value for -mem flag
	MAXMEMPERCENTDEFAULT = 60

	ERROR_NORESULTS = "Query returned no results"
)

// Direction indicates the counters of which flow direction we should print.
type Direction int

const (
	DIRECTION_SUM  Direction = iota // sum of inbound and outbound counters
	DIRECTION_IN                    // inbound counters
	DIRECTION_OUT                   // outbound counters
	DIRECTION_BOTH                  // inbound and outbound counters
)

// SortOrder indicates by what the entries are sorted.
type SortOrder int

const (
	SORT_PACKETS SortOrder = iota
	SORT_TRAFFIC
	SORT_TIME
)

// convenience wrapper around the summed counters
type Counts struct {
	PktsRcvd, PktsSent   uint64
	BytesRcvd, BytesSent uint64
}

// For the sorting we refer to closures to be able so sort by whatever value
// struct field we want
type Entry struct {
	k        goDB.ExtraKey
	nBr, nBs uint64
	nPr, nPs uint64
}

type by func(e1, e2 *Entry) bool

type entrySorter struct {
	entries []Entry
	less    func(e1, e2 *Entry) bool
}

// Sort is a method on the function type, By, that sorts the argument slice according to the function
func (b by) Sort(entries []Entry) {
	es := &entrySorter{
		entries: entries,
		less:    b, // closure for sort order defintion
	}
	sort.Sort(es)
}

// Len is part of sort.Interface.
func (s *entrySorter) Len() int {
	return len(s.entries)
}

// Swap is part of sort.Interface.
func (s *entrySorter) Swap(i, j int) {
	s.entries[i], s.entries[j] = s.entries[j], s.entries[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *entrySorter) Less(i, j int) bool {
	return s.less(&s.entries[i], &s.entries[j])
}

func By(sort SortOrder, direction Direction, ascending bool) by {
	switch sort {
	case SORT_PACKETS:
		switch direction {
		case DIRECTION_BOTH, DIRECTION_SUM:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nPs+e1.nPr < e2.nPs+e2.nPr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nPs+e1.nPr > e2.nPs+e2.nPr
				}
			}
		case DIRECTION_IN:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nPr < e2.nPr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nPr > e2.nPr
				}
			}
		case DIRECTION_OUT:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nPs < e2.nPs
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nPs > e2.nPs
				}
			}
		}
	case SORT_TRAFFIC:
		switch direction {
		case DIRECTION_BOTH, DIRECTION_SUM:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nBs+e1.nBr < e2.nBs+e2.nBr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nBs+e1.nBr > e2.nBs+e2.nBr
				}
			}
		case DIRECTION_IN:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nBr < e2.nBr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nBr > e2.nBr
				}
			}
		case DIRECTION_OUT:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nBs < e2.nBs
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nBs > e2.nBs
				}
			}
		}
	case SORT_TIME:
		if ascending {
			return func(e1, e2 *Entry) bool {
				return e1.k.Time < e2.k.Time
			}
		} else {
			return func(e1, e2 *Entry) bool {
				return e1.k.Time > e2.k.Time
			}
		}
	}

	panic("Failed to generate Less func for sorting entries")
}

func getPhysMem() (float64, error) {
	var memFile *os.File
	var ferr error
	if memFile, ferr = os.OpenFile("/proc/meminfo", os.O_RDONLY, 0444); ferr != nil {
		return 0.0, errors.New("Unable to open /proc/meminfo: " + ferr.Error())
	}

	physMem := 0.0
	memInfoScanner := bufio.NewScanner(memFile)
	for memInfoScanner.Scan() {
		if strings.Contains(memInfoScanner.Text(), "MemTotal") {
			memTokens := strings.Split(memInfoScanner.Text(), " ")
			physMem, _ = strconv.ParseFloat(memTokens[len(memTokens)-2], 64)
		}
	}

	if physMem < 0.1 {
		return 0.0, errors.New("Unable to obtain amount of physical memory from /proc/meminfo")
	}

	if ferr = memFile.Close(); ferr != nil {
		return 0.0, errors.New("Unable to close /proc/meminfo after reading: " + ferr.Error())
	}

	return physMem, nil
}

// Command line options parsing --------------------------------------------------
func ReadFlags(config *Config) error {

	flagSet := flag.NewFlagSet("goquery", flag.ContinueOnError)
	flagSet.Usage = func() { return }
	flag.ErrHelp = nil

	// Warning: The usage texts provided here are never printed.
	// All help that is shown to the user resides in goDB/DBHelp.go
	flagSet.StringVar(&config.Ifaces, "i", "", "Interfaces for which the query should be performed (e.g. 'eth0', 'eth0,t4_33760', 'ANY', ...)")
	flagSet.StringVar(&config.Conditions, "c", "", "Logical conditions for the query")
	flagSet.StringVar(&config.BaseDir, "d", "", "Path to database directory. By default the DB from the goProbe config is used")
	flagSet.BoolVar(&config.ListDB, "list", false, "lists on which interfaces data was captured and written to the DB")
	flagSet.StringVar(&config.Format, "e", "txt", "Output format: {txt|json|csv|influxdb}")
	flagSet.BoolVar(&config.Help, "h", false, "Prints the help page")
	flagSet.BoolVar(&config.Help, "help", false, "Prints the help page")
	flagSet.BoolVar(&config.HelpAdmin, "help-admin", false, "Prints the advanced help page")
	flagSet.BoolVar(&config.Version, "version", false, "Print version information and exit")
	flagSet.BoolVar(&config.WipeAdmin, "wipe", false, "wipes the entire database")
	flagSet.Int64Var(&config.CleanAdmin, "clean", 0, "cleans all entries before indicated timestamp")
	flagSet.BoolVar(&config.External, "x", false, "Mode for external calls, e.g. from portal")
	flagSet.StringVar(&config.Sort, "s", "bytes", "Sort results by accumulated packets instead of bytes")
	flagSet.BoolVar(&config.SortAscending, "a", false, "Sort results in ascending order")
	flagSet.BoolVar(&config.Incoming, "in", false, "Take into account incoming data (received packets/bytes)")
	flagSet.BoolVar(&config.Outgoing, "out", false, "Take into account outgoing data (sent packets/bytes)")
	flagSet.BoolVar(&config.Sum, "sum", false, "Sum incoming and outcoming data")
	flagSet.IntVar(&config.NumResults, "n", 1000, "Maximum number of final entries to show. Defaults to 95% of the overall data volume / number of packets (depending on the '-p' 60)")
	flagSet.BoolVar(&config.ShowMgmtTraffic, "m", false, "Show management traffic on port 5551")

	// Reverse DNS flags
	flagSet.BoolVar(&config.Resolve, "resolve", false, "Enable reverse DNS lookups in output")
	flagSet.DurationVar(&config.ResolveTimeout, "resolve-timeout", 1*time.Second, "Timeout for reverse DNS lookups")
	flagSet.IntVar(&config.ResolveRows, "resolve-rows", 25, "Maximum number of output rows to perform reverse DNS lookups on")

	// intentionally undocumented flag to set maximum percentage of physical memory
	// to be used before program terminates itself.
	flagSet.IntVar(&config.MaxMemPercent, "mem", MAXMEMPERCENTDEFAULT, "")

	// choose the last 30 days as default value for the time span, ensuring that
	// queries never run over data covering more than a month by default. Of course
	// the parameter can be overridden to cover longer time spans
	flagSet.StringVar(&config.First, "f", "-30d", "Lower bound on flow timestamp")
	flagSet.StringVar(&config.Last, "l", "9999999999999999", "Upper bound on flow timestamp")

	parseErr := flagSet.Parse(os.Args[1:])
	if parseErr != nil {
		return parseErr
	}

	// check if a DB path has been provided. If not, take the default one
	if config.BaseDir == "" {
		var dberr error
		config.BaseDir, dberr = getDefaultDBDir()
		if dberr != nil {
			return fmt.Errorf("could not get DBDir: %s", dberr.Error())
		}
	}

	// make sure that the interface was specified
	if strings.Contains(config.Ifaces, "-") {
		return errors.New("Interface not specified")
	}

	// setting the physical memory limit to more memory than the system has is pointless.
	if config.MaxMemPercent < 0 || 100 < config.MaxMemPercent {
		return errors.New("Invalid argument for -mem: Maxmimum memory percentage has to lie in interval [0; 100]")
	}

	if flagSet.NArg() > 1 {
		return errors.New("Query type must be the last (and only) argument of the call")
	}

	config.QueryType = (flagSet.Arg(0))

	// by default, we show incoming and outgoing traffic
	if !config.Incoming && !config.Outgoing && !config.Sum {
		config.Incoming, config.Outgoing = true, true
	}

	return nil
}

// Database cleanup functions ----------------------------------------------------
// completely removes all folders inside the base directory. Handle with care!
func wipeDB(dbPath string) error {
	// Get list of files in directory
	var dirList []os.FileInfo
	var err error

	if dirList, err = ioutil.ReadDir(dbPath); err != nil {
		return err
	}

	for _, file := range dirList {
		if file.IsDir() && (file.Name() != "./" || file.Name() != "../") {
			if rmerr := os.RemoveAll(dbPath + "/" + file.Name()); rmerr != nil {
				return rmerr
			}
		}
	}

	return err
}

// Message handling --------------------------------------------------------------
func throwMsg(msg string, external bool, fmtSpec string) {
	customer_text := "An error occurred while retrieving the requested information"
	out_level := os.Stderr
	status := "error"

	if strings.HasPrefix(msg, ERROR_NORESULTS) {
		out_level = os.Stdout
		status = "empty"
	}

	// If called interactively, show full error message to user (otherwise show
	// customer friendly generic message)
	if !(msg != ERROR_NORESULTS && external) {
		customer_text = msg
	}

	if fmtSpec == "json" {
		// If called non-interactively, write full error message to message log
		if external {
			if goDB.InitDBLog() != nil {
				return
			}
			goDB.SysLog.Err(msg)
		}
		message := map[string]string{
			"status":        status,
			"statusMessage": customer_text,
		}

		json_out, _ := json.Marshal(message)
		fmt.Fprintf(out_level, "%s", json_out)
	} else {
		fmt.Fprintf(out_level, "%s\n", customer_text)
	}
}

func parseIfaceList(dbPath string, ifacelist string) (ifaces []string, err error) {
	if ifacelist == "" {
		return nil, fmt.Errorf("No interface(s) specified")
	}

	if strings.ToLower(ifacelist) == "any" {
		summary, err := goDB.ReadDBSummary(dbPath)
		if err != nil {
			return nil, err
		}
		for iface, _ := range summary.Interfaces {
			ifaces = append(ifaces, iface)
		}
	} else {
		ifaces = strings.Split(ifacelist, ",")
		for _, iface := range ifaces {
			if strings.Contains(iface, "-") { // TODO: checking for "-" is kinda ugly
				err = fmt.Errorf("Invalid interface list")
				return
			}
		}
	}
	return
}

func createWorkManager(dbPath string, iface string, tfirst, tlast int64, query *goDB.Query, numProcessingUnits int) (workManager *goDB.DBWorkManager, nonempty bool, err error) {
	if workManager, err = goDB.NewDBWorkManager(dbPath, iface, numProcessingUnits); err != nil {
		return nil, false, fmt.Errorf("Could not initialize query workManager for interface '%s': %s", iface, err)
	}

	nonempty, err = workManager.CreateWorkerJobs(tfirst, tlast, query)
	return workManager, nonempty, err
}

type aggregateResult struct {
	aggregatedMap map[goDB.ExtraKey]goDB.Val
	totals        Counts
	err           error
}

// receive maps on mapChan until mapChan gets closed.
// Then send aggregation result over resultChan.
// If an error occurs, aggregate may return prematurely.
// Closes resultChan on termination.
func aggregate(mapChan <-chan map[goDB.ExtraKey]goDB.Val, resultChan chan<- aggregateResult) {
	defer close(resultChan)

	var finalMap = make(map[goDB.ExtraKey]goDB.Val)
	var totals Counts

	// Temporary goDB.Val because map values cannot be updated in-place
	var tempVal goDB.Val
	var exists bool

	// Create global MemStats object for tracking of memory consumption
	m := runtime.MemStats{}
	lastGC := time.Now()

	for item := range mapChan {
		if item == nil {
			resultChan <- aggregateResult{
				err: fmt.Errorf("Error during daily DB processing. Check syslog/messages for more information"),
			}
			return
		}
		for k, v := range item {
			totals.BytesRcvd += v.NBytesRcvd
			totals.BytesSent += v.NBytesSent
			totals.PktsRcvd += v.NPktsRcvd
			totals.PktsSent += v.NPktsSent

			if tempVal, exists = finalMap[k]; exists {
				tempVal.NBytesRcvd += v.NBytesRcvd
				tempVal.NBytesSent += v.NBytesSent
				tempVal.NPktsRcvd += v.NPktsRcvd
				tempVal.NPktsSent += v.NPktsSent

				finalMap[k] = tempVal
			} else {
				finalMap[k] = v
			}
		}

		item = nil

		// Conditionally call a manual garbage collection and memory release if the current heap allocation
		// is above GOGCLIMIT and more than GOGCINTERVAL seconds have passed
		runtime.ReadMemStats(&m)
		if m.Sys-m.HeapReleased > GOGCLIMIT && time.Since(lastGC) > GOGCINTERVAL {
			runtime.GC()
			debug.FreeOSMemory()
			lastGC = time.Now()
		}
	}

	if len(finalMap) == 0 {
		resultChan <- aggregateResult{
			err: fmt.Errorf(ERROR_NORESULTS),
		}
		return
	}

	resultChan <- aggregateResult{
		aggregatedMap: finalMap,
		totals:        totals,
	}
}

//--------------------------------------------------------------------------------
func main() {
	//--------------------------------------------------------------------------------

	// CPU Profiling Calls
	//    runtime.SetBlockProfileRate(10000000) // PROFILING DEBUG
	//    f, proferr := os.Create("GPCore.prof")    // PROFILING DEBUG
	//    if proferr != nil {                       // PROFILING DEBUG
	//        fmt.Println("Profiling error: "+proferr.Error()) // PROFILING DEBUG
	//    } // PROFILING DEBUG
	//    pprof.StartCPUProfile(f)     // PROFILING DEBUG
	//    defer pprof.StopCPUProfile() // PROFILING DEBUG

	/// CPU & THREADING SETTINGS ///
	numCpu := runtime.NumCPU()
	runtime.GOMAXPROCS(numCpu)

	/// WORKER SETTINGS
	// explicitly decouple numCpus an numProcessingUnits (but still use numCpu at the moment)
	numProcessingUnits := numCpu

	// Start timing
	tStart := time.Now()

	/// COMMAND LINE OPTIONS PARSING ///
	var queryConfig Config
	if parseErr := ReadFlags(&queryConfig); parseErr != nil {

		// We have to assume here that the call was interactive, since we don't know the
		// external / format options yet
		printHelpFlag := PrintFlagGenerator(false)
		printHelpFlag("")

		throwMsg(parseErr.Error(), false, "txt")
		return
	}

	// verify config format
	switch queryConfig.Format {
	case "txt", "csv", "json", "influxdb":
	default:
		throwMsg("Unknown output format", false, "txt")
		return
	}

	// define help printing functions
	printUsage := PrintUsageGenerator(queryConfig.External)
	printHelpFlag := PrintFlagGenerator(queryConfig.External)

	// check if only help needs to be printed
	if queryConfig.Help {
		printUsage()
		return
	}

	if queryConfig.HelpAdmin {
		printHelpFlag("admin")
		return
	}

	if queryConfig.Version {
		fmt.Printf("goProbe %s\n", version.VersionText())
		return
	}

	if queryConfig.ListDB {
		if lserr := listInterfaces(queryConfig.BaseDir, queryConfig.External); lserr != nil {
			throwMsg("Failed to retrieve list of available databases: "+lserr.Error(), queryConfig.External, queryConfig.Format)
		}
		return
	}

	// check if data needs to be cleaned or wiped
	if queryConfig.WipeAdmin {
		if wiperr := wipeDB(queryConfig.BaseDir); wiperr != nil {
			throwMsg("Failed to completely remove database: "+wiperr.Error(), queryConfig.External, queryConfig.Format)
			return
		}
		return
	}

	if queryConfig.CleanAdmin > 0 {
		if clerr := cleanOldDBDirs(queryConfig.BaseDir, queryConfig.CleanAdmin); clerr != nil {
			throwMsg("Database clean up failed: "+clerr.Error(), queryConfig.External, queryConfig.Format)
		}
		return
	}

	// We are in query mode.
	// Parse/check corresponding flags.

	ifaces, err := parseIfaceList(queryConfig.BaseDir, queryConfig.Ifaces)
	if err != nil {
		printHelpFlag("i")
		throwMsg(err.Error(), queryConfig.External, queryConfig.Format)
	}

	if queryConfig.Sort != "bytes" && queryConfig.Sort != "packets" && queryConfig.Sort != "time" {
		printHelpFlag("s")
		throwMsg("Incorrect sorting parameter specified", queryConfig.External, queryConfig.Format)
		return
	}

	if queryConfig.QueryType == "" {
		printHelpFlag("")
		throwMsg("No query type specified", queryConfig.External, queryConfig.Format)
		return
	}

	queryAttributes, hasAttrTime, hasAttrIface, err := goDB.ParseQueryType(queryConfig.QueryType)
	if err != nil {
		printHelpFlag("")
		throwMsg(err.Error(), queryConfig.External, queryConfig.Format)
	}

	// insert iface attribute here in case multiple interfaces where specified and the interface column
	// was not added as an attribute
	if (len(ifaces) > 1 || strings.Contains(queryConfig.Ifaces, "any")) && !strings.Contains(queryConfig.QueryType, "iface") {
		hasAttrIface = true
	}

	// If output format is influx, always take time with you
	if queryConfig.Format == "influxdb" {
		hasAttrTime = true
	}

	// override sorting direction and number of entries for time based queries
	if hasAttrTime {
		queryConfig.Sort = "time"
		queryConfig.SortAscending = true
		queryConfig.NumResults = 9999999999999999
	}

	// parse time bound
	var qcLast, qcFirst int64
	var lerr error

	if qcLast, lerr = goDB.ParseTimeArgument(queryConfig.Last); lerr != nil {
		printHelpFlag("f")
		throwMsg("Invalid time format: "+lerr.Error(), queryConfig.External, queryConfig.Format)
		return
	}
	if qcFirst, lerr = goDB.ParseTimeArgument(queryConfig.First); lerr != nil {
		printHelpFlag("f")
		throwMsg("Invalid time format: "+lerr.Error(), queryConfig.External, queryConfig.Format)
		return
	}

	if qcLast <= qcFirst {
		printHelpFlag("f")
		throwMsg("Invalid time interval: the lower time bound cannot be greater than the upper time bound"+queryConfig.QueryType, queryConfig.External, queryConfig.Format)
		return
	}

	// Obtain physical memory of this host
	var (
		physMem float64
		memErr  error
	)

	physMem, memErr = getPhysMem()
	if memErr != nil {
		throwMsg(memErr.Error(), queryConfig.External, queryConfig.Format)
		return
	}

	/// QUERY PREPARATION ///

	// If -x is specified, we exclude the management network range.
	// -x conflicts with the new -in -out behaviour, so we take -in -out to mean
	// -sum when -x is set.
	if queryConfig.External {
		queryConfig.Conditions = excludeManagementNet(queryConfig.Conditions)

		if queryConfig.Incoming && queryConfig.Outgoing {
			queryConfig.Sum, queryConfig.Incoming, queryConfig.Outgoing = true, false, false
		}
	}

	// hideOSAGManagementTraffic: if -m is not specified, will (not) add the static "no port 5551" filter
	if !queryConfig.ShowMgmtTraffic {
		queryConfig.Conditions = hideManagementTraffic(queryConfig.Conditions)
	}

	var direction Direction
	switch {
	case queryConfig.Sum:
		direction = DIRECTION_SUM
	case queryConfig.Incoming && !queryConfig.Outgoing:
		direction = DIRECTION_IN
	case !queryConfig.Incoming && queryConfig.Outgoing:
		direction = DIRECTION_OUT
	default:
		direction = DIRECTION_BOTH
	}

	var sortOrder SortOrder
	switch queryConfig.Sort {
	case "bytes":
		sortOrder = SORT_TRAFFIC
	case "time":
		sortOrder = SORT_TIME
	case "packets":
		fallthrough
	default:
		sortOrder = SORT_PACKETS
	}

	// sanitize conditional if one was provided
	var sanErr error
	queryConfig.Conditions, sanErr = goDB.SanitizeUserInput(queryConfig.Conditions)
	if sanErr != nil {
		printHelpFlag("c")
		throwMsg("Input sanitization error: "+sanErr.Error(), queryConfig.External, queryConfig.Format)
		return
	}

	// build condition tree to check if there is a syntax error before starting processing
	queryConditional, parseErr := goDB.ParseAndInstrumentConditional(queryConfig.Conditions, queryConfig.ResolveTimeout)
	if parseErr != nil {
		printHelpFlag("c")
		throwMsg("Condition error:\n"+parseErr.Error(), queryConfig.External, queryConfig.Format)
		return
	}

	query := goDB.NewQuery(queryAttributes, queryConditional, hasAttrTime, hasAttrIface)

	// Chek whether DNS works on this system
	if queryConfig.Resolve {
		if err := checkDNS(); err != nil {
			throwMsg("DNS warning: "+err.Error(), queryConfig.External, queryConfig.Format)
		}
	}

	/// DATA ACQUISITION AND PREPARATION ///

	// Start ticker to check memory consumption every second
	memTicker := time.NewTicker(MEMCHECKINTERVAL)
	go func() {
		m := runtime.MemStats{}
		for ; true; <-memTicker.C {
			runtime.ReadMemStats(&m)

			// Check if current memory consumption is higher than maximum allowed percentage of the available
			// physical memory
			if (m.Sys-m.HeapReleased)/1024 > uint64(float64(queryConfig.MaxMemPercent)*physMem/100) {
				memTicker.Stop()
				msg := fmt.Sprintf("Memory consumption above %v%% of physical memory. Aborting query", queryConfig.MaxMemPercent)
				throwMsg(msg, queryConfig.External, queryConfig.Format)
				os.Exit(1)
			}
		}
	}()

	// create work managers
	workManagers := map[string]*goDB.DBWorkManager{} // map interfaces to workManagers
	for _, iface := range ifaces {
		wm, nonempty, err := createWorkManager(queryConfig.BaseDir, iface, qcFirst, qcLast, query, numProcessingUnits)
		if err != nil {
			throwMsg(err.Error(), queryConfig.External, queryConfig.Format)
			return
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
	aggregateChan := make(chan aggregateResult, 1)
	go aggregate(mapChan, aggregateChan)

	// spawn reader processing units and make them work on the individual DB blocks
	for _, workManager := range workManagers {
		workManager.ExecuteWorkerReadJobs(mapChan)
	}
	// we are done with all worker jobs
	close(mapChan)

	agg := <-aggregateChan

	if agg.err != nil {
		throwMsg(agg.err.Error(), queryConfig.External, queryConfig.Format)
		return
	}

	/// DATA PRESENATION ///
	var mapEntries []Entry = make([]Entry, len(agg.aggregatedMap))
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
	if queryConfig.Format != "influxdb" {
		By(sortOrder, direction, queryConfig.SortAscending).Sort(mapEntries)
	}

	// Find map from ips to domains for reverse DNS
	var ips2domains map[string]string
	var resolveDuration time.Duration
	if queryConfig.Resolve && goDB.HasDNSAttributes(queryAttributes) {
		var ips []string
		var sip, dip goDB.Attribute
		for _, attribute := range queryAttributes {
			if attribute.Name() == "sip" {
				sip = attribute
			}
			if attribute.Name() == "dip" {
				dip = attribute
			}
		}

		for i, l := 0, len(mapEntries); i < l && i < queryConfig.ResolveRows; i++ {
			key := mapEntries[i].k
			if sip != nil {
				ips = append(ips, sip.ExtractStrings(&key)[0])
			}
			if dip != nil {
				ips = append(ips, dip.ExtractStrings(&key)[0])
			}
		}

		resolveStart := time.Now()
		ips2domains = timedReverseLookup(ips, queryConfig.ResolveTimeout)
		resolveDuration = time.Now().Sub(resolveStart)
	}

	// get the right printer
	var printer TablePrinter
	if printer, err = NewTablePrinter(
		queryConfig,
		sortOrder,
		direction,
		hasAttrTime, hasAttrIface,
		queryAttributes,
		ips2domains,
		agg.totals,
		count,
	); err != nil {
		throwMsg("Failed to create printer: "+err.Error(), queryConfig.External, queryConfig.Format)
		return
	}

	// stop timing everything related to the query
	tStop := time.Now()

	// fill the printer
	if queryConfig.NumResults < len(mapEntries) {
		mapEntries = mapEntries[:queryConfig.NumResults]
	}
	for _, entry := range mapEntries {
		printer.AddRow(entry)
	}

	printer.Footer(queryConfig.Conditions, tSpanFirst, tSpanLast, tStop.Sub(tStart), resolveDuration)

	// print the data
	if perr := printer.Print(); perr != nil {
		throwMsg(perr.Error(), queryConfig.External, queryConfig.Format)
		return
	}

	memTicker.Stop()

	return
}
