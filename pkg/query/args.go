package query

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/query/dns"
)

// Args bundles the command line/HTTP parameters required to prepare a query statement
type Args struct {
	// required
	Query  string // the query type such as sip,dip
	Ifaces string

	// data filtering
	Condition string

	// counter addition
	In  bool
	Out bool
	Sum bool

	// time selection
	First string
	Last  string

	// formatting
	Format        string
	SortBy        string // column to sort by (packets or bytes)
	NumResults    int
	External      bool
	SortAscending bool
	Output        string

	// do-and-exit flags
	List    bool
	Version bool

	// resolution
	Resolve        bool
	ResolveTimeout int
	ResolveRows    int

	// file system
	DBPath    string
	MaxMemPct int

	// stores who produced these args (caller)
	Caller string
}

// Prepare takes the query Arguments, validates them and creates an executable statement
func (a *Args) Prepare() (*Statement, error) {
	s := &Statement{
		QueryType:  a.Query,
		Resolve:    a.Resolve,
		Conditions: a.Condition,
		Caller:     a.Caller,
	}

	var err error

	// verify config format
	switch a.Format {
	case "txt", "csv", "json", "influxdb":
	default:
		return s, fmt.Errorf("unknown output format '%s'", a.Format)
	}
	s.Format = a.Format

	// check DB path
	err = CheckDBExists(a.DBPath)
	if err != nil {
		return s, err
	}
	s.DBPath = a.DBPath

	// assign ifaces
	s.Ifaces, err = parseIfaceList(s.DBPath, a.Ifaces)
	if err != nil {
		return s, fmt.Errorf("failed to parse interface list: %s", err)
	}

	// assign sort order and direction
	if !(a.SortBy == "bytes" || a.SortBy == "packets" || a.SortBy == "time") {
		return s, fmt.Errorf("unknown sorting parameter '%s' specified", a.SortBy)
	}
	switch a.SortBy {
	case "bytes":
		s.SortBy = SORT_TRAFFIC
	case "time":
		s.SortBy = SORT_TIME
	case "packets":
		fallthrough
	default:
		s.SortBy = SORT_PACKETS
	}

	var queryAttributes []goDB.Attribute
	queryAttributes, s.HasAttrTime, s.HasAttrIface, err = goDB.ParseQueryType(a.Query)
	if err != nil {
		return s, fmt.Errorf("failed to parse query type: %s", err)
	}

	// insert iface attribute here in case multiple interfaces where specified and the
	// interface column was not added as an attribute
	if (len(s.Ifaces) > 1 || strings.Contains(a.Ifaces, "any")) &&
		!strings.Contains(a.Query, "iface") {
		s.HasAttrIface = true
	}

	// If output format is influx, always take time with you
	if s.Format == "influxdb" {
		s.HasAttrTime = true
	}

	// override sorting direction and number of entries for time based queries
	if s.HasAttrTime {
		s.SortBy = SORT_TIME
		s.SortAscending = true
		s.NumResults = 9999999999999999
	}

	// parse time bound
	s.Last, err = goDB.ParseTimeArgument(a.Last)
	if err != nil {
		return s, fmt.Errorf("invalid time format for --last: %s", err)
	}
	s.First, err = goDB.ParseTimeArgument(a.First)
	if err != nil {
		return s, fmt.Errorf("invalid time format for --first: %s", err)
	}
	if s.Last <= s.First {
		return s, fmt.Errorf("invalid time interval: the lower time bound cannot be greater than the upper time bound")
	}

	// check external calls
	if a.External {
		a.Condition = excludeManagementNet(a.Condition)

		if a.In && a.Out {
			a.Sum, a.In, a.Out = true, false, false
		}
	}

	switch {
	case a.Sum:
		s.Direction = DIRECTION_SUM
	case a.In && !a.Out:
		s.Direction = DIRECTION_IN
	case !a.In && a.Out:
		s.Direction = DIRECTION_OUT
	default:
		s.Direction = DIRECTION_BOTH
	}

	// check resolve timeout and DNS
	if s.Resolve {
		err := dns.CheckDNS()
		if err != nil {
			return s, fmt.Errorf("DNS warning: %s", err)
		}
		if !(0 < a.ResolveTimeout) {
			return s, fmt.Errorf("resolve-timeout must be greater than 0")
		}
		s.ResolveTimeout = time.Duration(a.ResolveTimeout) * time.Second

		if !(0 < a.ResolveRows) {
			return s, fmt.Errorf("resolve-rows must be greater than 0")
		}
		s.ResolveRows = a.ResolveRows
	}

	// sanitize conditional if one was provided
	a.Condition, err = goDB.SanitizeUserInput(a.Condition)
	if err != nil {
		return s, fmt.Errorf("condition sanitization error: %s", err)
	}
	s.Conditions = a.Condition

	// build condition tree to check if there is a syntax error before starting processing
	queryConditional, parseErr := goDB.ParseAndInstrumentConditional(a.Condition, time.Duration(a.ResolveTimeout))
	if parseErr != nil {
		return s, fmt.Errorf("condition error: %s", parseErr)
	}

	// check memory flag
	if !(0 < a.MaxMemPct && a.MaxMemPct <= 100) {
		return s, fmt.Errorf("invalid memory percentage of '%d' provided", a.MaxMemPct)
	}
	s.MaxMemPct = a.MaxMemPct

	// check limits flag
	if !(0 < a.NumResults) {
		return s, fmt.Errorf("the printed row limit must be greater than 0")
	}
	s.NumResults = a.NumResults

	s.Query = goDB.NewQuery(queryAttributes, queryConditional, s.HasAttrTime, s.HasAttrIface)
	return s, nil
}

func parseIfaceList(dbPath string, ifacelist string) (ifaces []string, err error) {
	if ifacelist == "" {
		return nil, fmt.Errorf("no interface(s) specified")
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
				err = fmt.Errorf("interface list contains invalid character '-'")
				return
			}
		}
	}
	return
}

// CheckExistsDB will return nil if a DB at path exists and otherwise the error encountered
func CheckDBExists(path string) error {
	if path == "" {
		return fmt.Errorf("empty DB path provided")
	}
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("DB directory does not exist at %s", path)
		}
		return fmt.Errorf("failed to check DB directory: %s", err)
	}
	return nil
}
