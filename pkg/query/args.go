package query

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/conditions"
	"github.com/els0r/goProbe/pkg/goDB/conditions/node"
	"github.com/els0r/goProbe/pkg/query/dns"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
)

var maxTimeStr = fmt.Sprintf("%d", types.MaxTime.Unix())

// NewArgs creates new query arguments with the defaults set
func NewArgs(query, ifaces string, opts ...Option) *Args {
	a := DefaultArgs()

	// required args
	a.Query, a.Ifaces = query, ifaces

	// apply functional options
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// DefaultArgs creates a basic set of query arguments with only the
// defaults being set
func DefaultArgs() *Args {
	return &Args{
		First:      time.Now().AddDate(0, -1, 0).Format(time.ANSIC),
		Format:     DefaultFormat,
		In:         DefaultIn,
		Last:       maxTimeStr,
		MaxMemPct:  DefaultMaxMemPct,
		NumResults: DefaultNumResults,
		Out:        DefaultOut,
		DNSResolution: DNSResolution{
			MaxRows: DefaultResolveRows,
			Timeout: DefaultResolveTimeout,
		},
		SortBy: DefaultSortBy,
	}
}

// Args bundles the command line/HTTP parameters required to prepare a query statement
type Args struct {
	// required
	Query  string `json:"query" yaml:"query" form:"query"`    // Query: the query type. Example: sip,dip,dport,proto
	Ifaces string `json:"ifaces" yaml:"ifaces" form:"ifaces"` // Ifaces: the interfaces to query. Example: eth0,eth1

	QueryHosts string `json:"query_hosts,omitempty" yaml:"query_hosts,omitempty" form:"query_hosts,omitempty"` // QueryHosts: the hosts for which data is queried (comma-separated list). Example: hostA,hostB,hostC

	Hostname string `json:"hostname,omitempty" yaml:"hostname,omitempty" form:"hostname,omitempty"` // Hostname: the hostname from which data is queried. Example: localhost
	HostID   uint   `json:"host_id,omitempty" yaml:"host_id,omitempty" form:"host_id,omitempty"`    // HostID: the host id from which data is queried. Example: 123456

	// data filtering
	Condition string `json:"condition,omitempty" yaml:"condition,omitempty" form:"condition,omitempty"` // Condition: the condition to filter data by. Example: port=80 && proto=TCP

	// counter addition
	In  bool `json:"in,omitempty" yaml:"in,omitempty" form:"in,omitempty"`     // In: only show incoming packets/bytes. Example: false
	Out bool `json:"out,omitempty" yaml:"out,omitempty"  form:"out,omitempty"` // Out: only show outgoing packets/bytes. Example: false
	Sum bool `json:"sum,omitempty" yaml:"sum,omitempty" form:"sum,omitempty"`  // Sum: show sum of incoming/outgoing packets/bytes. Example: false

	// time selection
	First string `json:"first,omitempty" yaml:"first,omitempty" form:"first,omitempty"` // First: the first timestamp to query. Example: 2020-08-12T09:47:00+0200
	Last  string `json:"last,omitempty" yaml:"last,omitempty" form:"last,omitempty"`    // Last: the last timestamp to query. Example: -24h

	// formatting
	Format        string `json:"format,omitempty" yaml:"format,omitempty" form:"format,omitempty"`                         // Format: the output format. Enum: [json, csv, table]. Example: json
	SortBy        string `json:"sort_by,omitempty" yaml:"sort_by,omitempty" form:"sort_by,omitempty"`                      // SortBy: column to sort by. Enum: [packets, bytes]. Example: bytes
	NumResults    uint64 `json:"num_results,omitempty" yaml:"num_results,omitempty" form:"num_results,omitempty"`          // NumResults: number of results to return/print. Example: 25
	SortAscending bool   `json:"sort_ascending,omitempty" yaml:"sort_ascending,omitempty" form:"sort_ascending,omitempty"` // SortAscending: sort ascending instead of the default descending. Example: false

	// do-and-exit arguments
	List    bool `json:"list,omitempty" yaml:"list,omitempty" form:"list,omitempty"`          // List: only list interfaces and return. Example: false
	Version bool `json:"version,omitempty" yaml:"version,omitempty" form:"version,omitempty"` // Version: only print version and return. Example: false

	// resolution
	// Note: Nested structures are not supported for form data, see individual parameters in definition of DNSResolution
	DNSResolution DNSResolution `json:"dns_resolution,omitempty" yaml:"dns_resolution,omitempty"` // DNSResolution: guide reverse DNS resolution of sip,dip results

	// file system
	MaxMemPct int  `json:"max_mem_pct,omitempty" yaml:"max_mem_pct,omitempty" form:"max_mem_pct,omitempty"` // MaxMemPct: maximum percentage of available host memory to use for query processing. Example: 80
	LowMem    bool `json:"low_mem,omitempty" yaml:"low_mem,omitempty" form:"low_mem,omitempty"`             // LowMem: use less memory for query processing. Example: false

	// Caller stores who produced these args (caller). Example: goQuery. Example: goQuery. Example: goQuery. Example: goQuery
	Caller string `json:"caller,omitempty" yaml:"caller,omitempty" form:"caller,omitempty"`

	// Live can be used to request live flow data (in addition to DB results). Example: false
	Live bool `json:"live,omitempty" yaml:"live,omitempty" form:"live,omitempty"`

	// outputs is unexported
	outputs []io.Writer
}

// ArgsError provides a more detailed error description for invalid query args
type ArgsError struct {
	Field   string // Field: the string describing which field led to the error. It MUST match the json definition for a field
	Type    string // Type: the type of the error. Example: *types.ParseError
	Message string // Message: a human-readable, UI friendly description of the error. Example: Condition parsing failed
	err     error
}

func newArgsError(field string, msg string, err error) *ArgsError {
	args := &ArgsError{
		Field:   field,
		Message: msg,
		err:     err,
	}
	if err != nil {
		args.Type = fmt.Sprintf("%T", err)

		// make sure the type of the wrapped error is found out
		e := errors.Unwrap(err)
		if e != nil {
			args.Type = fmt.Sprintf("%T", e)
		}
	}
	return args
}

func (err *ArgsError) String() string {
	return fmt.Sprintf("%s: %s: (%s: %s)", err.Field, err.Message, err.Type, err.err)
}

// Error implements the error interface
func (err *ArgsError) Error() string {
	return err.String()
}

// Unwrap makes the error wrappable
func (err *ArgsError) Unwrap() error {
	return err.err
}

// LogValue implements slog.LogValuer
func (err *ArgsError) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("field", err.Field),
		slog.String("type", string(err.Type)),
		slog.String("message", err.Message),
		slog.Any("details", err.err),
	)
}

type marshallableError struct {
	error
}

func (m *marshallableError) MarshalJSON() ([]byte, error) {
	return jsoniter.Marshal(m.Error())
}

func (err *ArgsError) MarshalJSON() ([]byte, error) {
	var m = struct {
		Field   string `json:"field"`
		Type    string `json:"type,omitempty"`
		Message string `json:"message,omitempty"`
		Details any    `json:"details,omitempty"`
	}{Field: err.Field, Type: err.Type, Message: err.Message}

	if err.err == nil {
		m.Details = nil
		return jsoniter.Marshal(&m)
	}

	// check if the underlying error is a parse error
	e := errors.Unwrap(err.err)
	if e == nil {
		e = err.err
		m.Type = fmt.Sprintf("%T", e)
	}

	// need assertion because error doesn't know how to deal with marshalling
	switch t := e.(type) {
	case *types.ParseError,
		*types.MinBoundsError,
		*types.MaxBoundsError,
		*types.RangeError,
		*types.UnsupportedError:
		m.Details = t
	default:
		m.Details = &marshallableError{e}
	}

	return jsoniter.Marshal(&m)
}

// Pretty implements the Prettier interface to represent the error in a human-readable way
func (err *ArgsError) Pretty() string {
	str := `
  Field:   %s
  Message: %s
  Details: %s
`
	errStr := err.err.Error()

	prettyErr, ok := err.err.(types.Prettier)
	if ok {
		errStr = "\n" + types.PrettyIndent(prettyErr, 4)
	}

	return fmt.Sprintf(str, err.Field, err.Message, errStr)
}

// DNSResolution contains DNS query / resolution related config arguments / parameters
type DNSResolution struct {
	Enabled bool          `json:"enabled" yaml:"enabled" form:"dns_enabled"`                                  // Enabled: enable reverse DNS lookups. Example: false
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty" form:"dns_timeout,omitempty"`    // Timeout: timeout for reverse DNS lookups. Example: 2s
	MaxRows int           `json:"max_rows,omitempty" yaml:"max_rows,omitempty" form:"dns_max_rows,omitempty"` // MaxRows: maximum number of rows to resolve. Example: 100
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

// LogValue creates an slog compatible value from an Args instance
func (a *Args) LogValue() slog.Value {
	val := "<marshal failed>"
	b, err := jsoniter.Marshal(a)
	if err == nil {
		val = string(b)
	}
	return slog.StringValue(val)
}

const (
	invalidQueryTypeMsg            = "invalid query type"
	invalidFormatMsg               = "unknown format"
	invalidSortByMsg               = "unknown format"
	invalidTimeRangeMsg            = "invalid time range"
	invalidDNSResolutionTimeoutMsg = "invalid resolution timeout"
	invalidDNSResolutionRowsMsg    = "invalid number of rows"
	invalidConditionMsg            = "invalid condition"
	invalidMaxMemPctMsg            = "invalid max memory percentage"
	invalidRowLimitMsg             = "invalid row limit"
	invalidLiveQueryMsg            = "query not possible"
)

// Prepare takes the query Arguments, validates them and creates an executable statement. Optionally, additional writers can be passed to route query results to different destinations.
func (a *Args) Prepare(writers ...io.Writer) (*Statement, error) {
	var err error

	s := &Statement{
		QueryType:     a.Query,
		DNSResolution: a.DNSResolution,
		Condition:     a.Condition,
		LowMem:        a.LowMem,
		Caller:        a.Caller,
		Live:          a.Live,
		Output:        os.Stdout, // by default, we write results to the console
	}

	// the query type is parsed here already in order to validate if the query contains
	// errors
	var selector types.LabelSelector
	s.attributes, selector, err = types.ParseQueryType(a.Query)
	if err != nil {
		return s, newArgsError(
			"query",
			invalidQueryTypeMsg,
			err,
		)
	}

	// verify config format
	_, verifies := permittedFormats[a.Format]
	if !verifies {
		return s, newArgsError(
			"format",
			invalidFormatMsg,
			types.NewUnsupportedError(a.Format, PermittedFormats()),
		)
	}
	s.Format = a.Format

	// if not already done beforehand, enforce defaults for args
	if a.SortBy == "" {
		a.SortBy = "packets"
	}

	// assign sort order and direction
	s.SortBy, verifies = permittedSortBy[a.SortBy]
	if !verifies {
		return s, newArgsError(
			"sort_by",
			invalidSortByMsg,
			types.NewUnsupportedError(a.SortBy, PermittedSortBy()),
		)

	}

	// insert iface attribute here in case multiple interfaces where specified and the
	// interface column was not added as an attribute
	if (len(s.Ifaces) > 1 || strings.Contains(a.Ifaces, types.AnySelector)) &&
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
		return s, newArgsError(
			"first/last",
			invalidTimeRangeMsg,
			err,
		)
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
		// TODO: make this function available in the public domain or skip
		err := dns.CheckDNS()
		if err != nil {
			return s, newArgsError(
				"dns_resolution.enabled",
				"DNS check failed",
				err,
			)
		}
		if !(0 < s.DNSResolution.Timeout) {
			return s, newArgsError(
				"dns_resolution.timeout",
				invalidDNSResolutionTimeoutMsg,
				types.NewMinBoundsError(strconv.Itoa(int(s.DNSResolution.Timeout)), "0", false),
			)
		}
		if !(0 < s.DNSResolution.MaxRows) {
			return s, newArgsError(
				"dns_resolution.max_rows",
				invalidDNSResolutionRowsMsg,
				types.NewMinBoundsError(strconv.Itoa(int(s.DNSResolution.MaxRows)), "0", false),
			)
		}
	}

	// sanitize conditional if one was provided
	s.Condition = conditions.SanitizeUserInput(a.Condition)

	// build condition tree to check if there is a syntax error before starting processing
	_, _, parseErr := node.ParseAndInstrument(s.Condition, s.DNSResolution.Timeout)
	if parseErr != nil {
		return s, newArgsError(
			"condition",
			invalidConditionMsg,
			parseErr,
		)
	}

	// if we got here, the condition can definitely be tokenized. This makes sure the canonical
	// form of the condition is stored
	tokens, _ := conditions.Tokenize(s.Condition)
	s.Condition = strings.Join(tokens, " ")

	// check memory flag
	if !(0 < a.MaxMemPct && a.MaxMemPct <= 100) {
		return s, newArgsError(
			"max_mem_pct",
			invalidMaxMemPctMsg,
			types.NewRangeError(strconv.Itoa(a.MaxMemPct), "0", false, "100", true),
		)
	}
	s.MaxMemPct = a.MaxMemPct

	// check limits flag
	if a.NumResults <= 0 {
		return s, newArgsError(
			"num_results",
			invalidRowLimitMsg,
			types.NewMinBoundsError(strconv.Itoa(int(s.NumResults)), "0", false),
		)
	}
	s.NumResults = a.NumResults

	// check for consistent use of the live flag
	if s.Live && s.Last != types.MaxTime.Unix() {
		return s, newArgsError(
			"live",
			invalidLiveQueryMsg,
			errors.New("last timestamp unsupported"),
		)
	}

	// fan-out query results in case multiple writers were supplied
	writers = append(writers, a.outputs...)
	if len(writers) > 0 {
		s.Output = io.MultiWriter(writers...)
	}

	return s, nil
}
