package query

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/conditions"
	"github.com/els0r/goProbe/pkg/query/dns"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"golang.org/x/exp/slog"
)

// NewArgs creates new query arguments with the defaults set
func NewArgs(query, ifaces string, opts ...Option) *Args {
	a := &Args{
		// required args
		Query:  query,
		Ifaces: ifaces,

		// defaults
		First:      time.Now().AddDate(0, -1, 0).Format(time.ANSIC),
		Format:     DefaultFormat,
		In:         DefaultIn,
		Last:       time.Now().Format(time.ANSIC),
		MaxMemPct:  DefaultMaxMemPct,
		NumResults: DefaultNumResults,
		Out:        DefaultOut,
		DNSResolution: DNSResolution{
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
	Query  string `json:"query" yaml:"query"` // the query type such as sip,dip
	Ifaces string `json:"ifaces" yaml:"ifaces"`

	HostQuery string `json:"host_query,omitempty" yaml:"host_query,omitempty"` // the hosts query

	Hostname string `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	HostID   uint   `json:"host_id,omitempty" yaml:"host_id,omitempty"`

	// data filtering
	Condition string `json:"condition,omitempty" yaml:"condition,omitempty"`

	// counter addition
	In  bool `json:"in,omitempty" yaml:"in,omitempty"`
	Out bool `json:"out,omitempty" yaml:"out,omitempty"`
	Sum bool `json:"sum,omitempty" yaml:"sum,omitempty"`

	// time selection
	First string `json:"first,omitempty" yaml:"first,omitempty"`
	Last  string `json:"last,omitempty" yaml:"last,omitempty"`

	// formatting
	Format        string `json:"format,omitempty" yaml:"format,omitempty"`
	SortBy        string `json:"sort_by,omitempty" yaml:"sort_by,omitempty"` // column to sort by (packets or bytes)
	NumResults    int    `json:"num_results,omitempty" yaml:"num_results,omitempty"`
	SortAscending bool   `json:"sort_ascending,omitempty" yaml:"sort_ascending,omitempty"`
	External      bool   `json:"external,omitempty" yaml:"external,omitempty"`

	// do-and-exit arguments
	List    bool `json:"list,omitempty" yaml:"list,omitempty"`
	Version bool `json:"version,omitempty" yaml:"version,omitempty"`

	// resolution
	DNSResolution DNSResolution `json:"dns_resolution,omitempty" yaml:"dns_resolution,omitempty"`

	// file system
	MaxMemPct int  `json:"max_mem_pct,omitempty" yaml:"max_mem_pct,omitempty"`
	LowMem    bool `json:"low_mem,omitempty" yaml:"low_mem,omitempty"`

	// stores who produced these args (caller)
	Caller string `json:"caller,omitempty" yaml:"caller,omitempty"`

	// outputs is unexported
	outputs []io.Writer
}

type DNSResolution struct {
	Enabled bool          `json:"enabled" yaml:"enabled"`
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	MaxRows int           `json:"max_rows,omitempty" yaml:"max_rows,omitempty"`
}

// AddOutputs allows more control over to which outputs the
// query results are written
func (a *Args) AddOutputs(outputs ...io.Writer) *Args {
	a.outputs = outputs
	return a
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
	str += fmt.Sprintf(", limit: %d, from: %s, to: %s",
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

func (a *Args) LogValue() slog.Value {
	val := "<marshal failed>"
	b, err := json.Marshal(a)
	if err == nil {
		val = string(b)
	}
	return slog.StringValue(val)
}

// Prepare takes the query Arguments, validates them and creates an executable statement. Optionally, additional writers can be passed to route query results to different destinations.
func (a *Args) Prepare(writers ...io.Writer) (*Statement, error) {
	// if not already done beforehand, enforce defaults for args
	if a.SortBy == "" {
		a.SortBy = "packets"
	}

	s := &Statement{
		QueryType:     a.Query,
		DNSResolution: a.DNSResolution,
		Condition:     a.Condition,
		LowMem:        a.LowMem,
		Caller:        a.Caller,
		Output:        os.Stdout, // by default, we write results to the console
	}

	var err error

	// verify config format
	_, verifies := PermittedFormats[a.Format]
	if !verifies {
		return s, fmt.Errorf("unknown output format '%s'", a.Format)
	}
	s.Format = a.Format

	// assign sort order and direction
	s.SortBy, verifies = PermittedSortBy[a.SortBy]
	if !verifies {
		return s, fmt.Errorf("unknown sorting parameter '%s' specified", a.SortBy)
	}

	// the query type is parsed here already in order to validate if the query contains
	// errors
	var selector types.LabelSelector
	s.attributes, selector, err = types.ParseQueryType(a.Query)
	if err != nil {
		return s, fmt.Errorf("failed to parse query type: %w", err)
	}

	// insert iface attribute here in case multiple interfaces where specified and the
	// interface column was not added as an attribute
	if (len(s.Ifaces) > 1 || strings.Contains(a.Ifaces, "any")) &&
		!strings.Contains(a.Query, "iface") {
		selector.Iface = true
	}
	s.LabelSelector = selector

	// override sorting direction and number of entries for time based queries
	if selector.Timestamp {
		s.SortBy = results.SortTime
		s.SortAscending = true
		s.NumResults = MaxResults
	}

	// parse time bound
	s.First, s.Last, err = ParseTimeRange(a.First, a.Last)
	if err != nil {
		return s, err
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

	// fan-out query results in case multiple writers were supplied
	writers = append(writers, a.outputs...)
	if len(writers) > 0 {
		s.Output = io.MultiWriter(writers...)
	}

	return s, nil
}
