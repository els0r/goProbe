package query

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/els0r/goProbe/pkg/conditions"
	"github.com/els0r/goProbe/pkg/goDB/info"
	"github.com/els0r/goProbe/pkg/query/dns"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
)

// NewArgs creates new query arguments with the defaults set
func NewArgs(query, ifaces string, opts ...Option) *Args {
	a := &Args{
		// required args
		Query:  query,
		Ifaces: ifaces,

		// defaults
		DBPath:     DefaultDBPath,
		First:      time.Now().AddDate(0, -1, 0).Format(time.ANSIC),
		Format:     DefaultFormat,
		In:         DefaultIn,
		Last:       time.Now().Format(time.ANSIC),
		MaxMemPct:  DefaultMaxMemPct,
		NumResults: DefaultNumResults,
		Out:        DefaultOut,
		DNSResolution: struct {
			Enabled bool
			Timeout time.Duration
			MaxRows int
		}{
			MaxRows: DefaultResolveRows,
			Timeout: DefaultResolveTimeout,
		},
		SortBy: DefaultSortBy,
	}

	// apply functional options
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Args bundles the command line/HTTP parameters required to prepare a query statement
type Args struct {
	// required
	Query  string // the query type such as sip,dip
	Ifaces string

	Hostname string
	HostID   uint

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

	// do-and-exit arguments
	List    bool
	Version bool

	// resolution
	DNSResolution struct {
		Enabled bool
		Timeout time.Duration
		MaxRows int
	}

	// file system
	DBPath    string
	MaxMemPct int
	LowMem    bool

	// stores who produced these args (caller)
	Caller string

	// query is aborted after timeout expires
	QueryTimeout time.Duration
}

// String formats aruguments in human-readable form
func (a *Args) String() string {
	str := fmt.Sprintf("{type: %s, ifaces: %s",
		a.Query,
		a.Ifaces,
	)
	if a.Condition != "" {
		str += fmt.Sprintf(", condition: %s", a.Condition)
	}
	str += fmt.Sprintf(", db: %s, limit: %d, from: %s, to: %s",
		a.DBPath,
		a.NumResults,
		a.First,
		a.Last,
	)
	if a.DNSResolution.Enabled {
		str += fmt.Sprintf(", dns-resolution: %t, dns-timeout: %s, dns-rows-resolved: %d",
			a.DNSResolution.Enabled, a.DNSResolution.Timeout.Round(time.Second), a.DNSResolution.MaxRows,
		)
	}
	if a.Caller != "" {
		str += fmt.Sprintf(", caller: %s", a.Caller)
	}
	str += "}"
	return str
}

// Prepare takes the query Arguments, validates them and creates an executable statement. Optionally, additional writers can be passed to route query results to different destinations.
func (a *Args) Prepare(writers ...io.Writer) (*Statement, error) {
	// if not already done beforehand, enforce defaults for args
	if a.SortBy == "" {
		a.SortBy = "packets"
	}

	s := &Statement{
		QueryType: a.Query,
		DNSResolution: struct {
			Enabled bool          `json:"enabled,omitempty"`
			Timeout time.Duration `json:"dns_timeout,omitempty"`
			MaxRows int           `json:"max_rows,omitempty"`
		}(a.DNSResolution),
		Condition: a.Condition,
		LowMem:    a.LowMem,
		Caller:    a.Caller,
		Output:    os.Stdout, // by default, we write results to the console
	}

	var err error

	// verify config format
	_, verifies := PermittedFormats[a.Format]
	if !verifies {
		return s, fmt.Errorf("unknown output format '%s'", a.Format)
	}
	s.Format = a.Format

	s.DBPath = a.DBPath

	// assign ifaces
	s.Ifaces, err = parseIfaceList(s.DBPath, a.Ifaces)
	if err != nil {
		return s, fmt.Errorf("failed to parse interface list: %w", err)
	}

	// assign sort order and direction
	s.SortBy, verifies = PermittedSortBy[a.SortBy]
	if !verifies {
		return s, fmt.Errorf("unknown sorting parameter '%s' specified", a.SortBy)
	}

	_, _, _, err = types.ParseQueryType(a.Query)
	if err != nil {
		return s, fmt.Errorf("failed to parse query type: %w", err)
	}

	// insert iface attribute here in case multiple interfaces where specified and the
	// interface column was not added as an attribute
	if (len(s.Ifaces) > 1 || strings.Contains(a.Ifaces, "any")) &&
		!strings.Contains(a.Query, "iface") {
		s.HasAttrIface = true
	}

	// override sorting direction and number of entries for time based queries
	if s.HasAttrTime {
		s.SortBy = results.SortTime
		s.SortAscending = true
		s.NumResults = MaxResults
	}

	// parse time bound
	s.Last, err = ParseTimeArgument(a.Last)
	if err != nil {
		return s, fmt.Errorf("invalid time format for --last: %w", err)
	}
	s.First, err = ParseTimeArgument(a.First)
	if err != nil {
		return s, fmt.Errorf("invalid time format for --first: %w", err)
	}
	if s.Last <= s.First {
		return s, fmt.Errorf("invalid time interval: the lower time bound cannot be greater than the upper time bound")
	}

	// check external calls
	if a.External {
		a.Condition = results.ExcludeManagementNet(a.Condition)

		if a.In && a.Out {
			a.Sum, a.In, a.Out = true, false, false
		}
	}

	switch {
	case a.Sum:
		s.Direction = types.DirectionSum
	case a.In && !a.Out:
		s.Direction = types.DirectionIn
	case !a.In && a.Out:
		s.Direction = types.DirectionOut
	default:
		s.Direction = types.DirectionBoth
	}

	// check resolve timeout and DNS
	if s.DNSResolution.Enabled {
		err := dns.CheckDNS()
		if err != nil {
			return s, fmt.Errorf("DNS warning: %w", err)
		}
		if !(0 < s.DNSResolution.Timeout) {
			return s, fmt.Errorf("resolve-timeout must be greater than 0")
		}
		if !(0 < s.DNSResolution.MaxRows) {
			return s, fmt.Errorf("resolve-rows must be greater than 0")
		}
	}

	// sanitize conditional if one was provided
	a.Condition, err = conditions.SanitizeUserInput(a.Condition)
	if err != nil {
		return s, fmt.Errorf("failed to sanitize condition: %w", err)
	}
	s.Condition = a.Condition

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

	// handling of the output field
	if a.Output != "" {
		// check if multiple files were specified
		outputs := strings.Split(a.Output, ",")

		for _, output := range outputs {
			// open file to write to
			queryFile, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0755)
			if err != nil {
				return s, fmt.Errorf("failed to open output file: %s", err)
			}
			writers = append(writers, queryFile)
		}
	}

	// fan-out query results in case multiple writers were supplied
	if len(writers) > 0 {
		s.Output = io.MultiWriter(writers...)
	}

	return s, nil
}

func parseIfaceList(dbPath string, ifacelist string) (ifaces []string, err error) {
	if ifacelist == "" {
		return nil, fmt.Errorf("no interface(s) specified")
	}

	if strings.ToLower(ifacelist) == "any" {
		ifaces, err = info.GetInterfaces(dbPath)
		if err != nil {
			return nil, err
		}
	} else {
		ifaces = strings.Split(ifacelist, ",")
		for _, iface := range ifaces {
			if iface == "" {
				err = fmt.Errorf("interface list contains empty interface name")
				return
			}
		}
	}
	return
}
