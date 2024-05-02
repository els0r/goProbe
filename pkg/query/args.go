package query

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
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
	// Query: the query type
	Query string `json:"query" yaml:"query" form:"query" doc:"Query type / Attributes to aggregate by" example:"sip,dip,dport,proto" minLength:"3"`
	// Ifaces: the interfaces to query
	Ifaces string `json:"ifaces" yaml:"ifaces" form:"ifaces" doc:"Interfaces to query" example:"eth0,eth1" minLength:"2"`

	// QueryHosts: the hosts for which data is queried (comma-separated list)
	QueryHosts string `json:"query_hosts,omitempty" yaml:"query_hosts,omitempty" form:"query_hosts,omitempty" doc:"Hosts for which data is queried" example:"hostA,hostB,hostC"`

	// Hostname: the hostname from which data is queried
	Hostname string `json:"hostname,omitempty" yaml:"hostname,omitempty" form:"hostname,omitempty" doc:"Hostname from which data is queried" example:"hostA"`
	// HostID: the host id from which data is queried
	HostID uint `json:"host_id,omitempty" yaml:"host_id,omitempty" form:"host_id,omitempty" doc:"Host ID from which data is queried" example:"123456"`

	// data filtering
	// Condition: the condition to filter data by
	Condition string `json:"condition,omitempty" yaml:"condition,omitempty" form:"condition,omitempty" doc:"Condition to filter data by" example:"port=80 & proto=TCP"`

	// counter addition
	// In: only show incoming packets/bytes
	In bool `json:"in,omitempty" yaml:"in,omitempty" form:"in,omitempty" doc:"Only show incoming packets/bytes" example:"false"`
	// Out: only show outgoing packets/bytes
	Out bool `json:"out,omitempty" yaml:"out,omitempty"  form:"out,omitempty" doc:"Only show outgoing packets/bytes" example:"false"`
	// Sum: show sum of incoming/outgoing packets/bytes
	Sum bool `json:"sum,omitempty" yaml:"sum,omitempty" form:"sum,omitempty" doc:"Show sum of incoming/outgoing packets/bytes" example:"false"`

	// time selection
	// First: the first timestamp to query
	First string `json:"first,omitempty" yaml:"first,omitempty" form:"first,omitempty" doc:"The first timestamp to query" example:"2020-08-12T09:47:00+02:00"`
	// Last: the last timestamp to query
	Last string `json:"last,omitempty" yaml:"last,omitempty" form:"last,omitempty" doc:"The last timestamp to query" example:"-24h"`

	// formatting
	// Format: the output format
	Format string `json:"format,omitempty" yaml:"format,omitempty" form:"format,omitempty" doc:"Output format" enum:"json,table,csv" example:"json"`
	// SortBy: column to sort by
	SortBy string `json:"sort_by,omitempty" yaml:"sort_by,omitempty" form:"sort_by,omitempty" doc:"Colum to sort by" enum:"bytes,packets" example:"packets" default:"bytes"`
	// NumResults: number of results to return/print
	NumResults uint64 `json:"num_results,omitempty" yaml:"num_results,omitempty" form:"num_results,omitempty" doc:"Number of results to return/print" example:"25" minimum:"1" default:"1000"`
	// SortAscending: sort ascending instead of the default descending
	SortAscending bool `json:"sort_ascending,omitempty" yaml:"sort_ascending,omitempty" form:"sort_ascending,omitempty" doc:"Sort ascending instead of descending" example:"false"`

	// do-and-exit arguments
	// List: only list interfaces and return
	List bool `json:"list,omitempty" yaml:"list,omitempty" form:"list,omitempty" hidden:"true"`
	// Version: only print version and return
	Version bool `json:"version,omitempty" yaml:"version,omitempty" form:"version,omitempty" hidden:"true"`

	// resolution
	// Note: Nested structures are not supported for form data, see individual parameters in definition of DNSResolution
	// DNSResolution: guide reverse DNS resolution of sip,dip results
	DNSResolution DNSResolution `json:"dns_resolution,omitempty" yaml:"dns_resolution,omitempty" doc:"Configures DNS resolution of sip,dip results"`

	// file system
	// MaxMemPct: maximum percentage of available host memory to use for query processing
	MaxMemPct int `json:"max_mem_pct,omitempty" yaml:"max_mem_pct,omitempty" form:"max_mem_pct,omitempty" doc:"Maximum percentage of available host memory to use for query processing" default:"60" example:"80" minimum:"1" maximum:"100"`
	// LowMem: use less memory for query processing
	LowMem bool `json:"low_mem,omitempty" yaml:"low_mem,omitempty" form:"low_mem,omitempty" doc:"Use less memory for query processing" example:"false"`

	// Caller stores who produced these args (caller)
	Caller string `json:"caller,omitempty" yaml:"caller,omitempty" form:"caller,omitempty" doc:"Caller stores who produced the arguments" example:"goQuery"`

	// Live can be used to request live flow data (in addition to DB results)
	Live bool `json:"live,omitempty" yaml:"live,omitempty" form:"live,omitempty" doc:"Live can be used to request live flow data (in addition to DB results)" example:"false"`

	// outputs is unexported
	outputs []io.Writer
}

// Pretty implements the Prettier interface to represent the error in a human-readable way
// TODO: prettify huma details error
// func (err *ArgsError) Pretty() string {
// 	str := `
//   Field:   %s
//   Message: %s
//   Details: %s
// `
// 	errStr := err.err.Error()

//	return fmt.Sprintf(str, err.Field, err.Message, errStr)
//
// '}
type DetailError struct {
	huma.ErrorModel
}

// Pretty implements the prettier interface for a huma.ErrorModel
func (d *DetailError) Pretty() string {
	var details []string
	for _, detail := range d.Errors {
		heading := fmt.Sprintf("%s (value: %v)", strings.TrimLeft(detail.Location, "body."), detail.Value)
		dashes := strings.Repeat("-", len(heading))

		details = append(details,
			fmt.Sprintf(`
%s
%s
%s`,
				heading, dashes, detail.Message,
			),
		)
	}

	return fmt.Sprintf("%s", strings.Join(details, "\n"))
}

// DNSResolution contains DNS query / resolution related config arguments / parameters
type DNSResolution struct {
	_ struct{} `nullable:"true"`
	// Enabled: enable reverse DNS lookups. Example: false
	Enabled bool `json:"enabled" yaml:"enabled" form:"dns_enabled" doc:"Enable reverse DNS lookups" example:"false"`
	// Timeout: timeout for reverse DNS lookups
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty" form:"dns_timeout,omitempty" doc:"Timeout for reverse DNS lookups" example:"2000000000" minimum:"0" default:"1000000000"`
	// MaxRows: maximum number of rows to resolve
	MaxRows int `json:"max_rows,omitempty" yaml:"max_rows,omitempty" form:"dns_max_rows,omitempty" doc:"Maximum number of rows to resolve" minimum:"1" example:"25"`
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

// ToJSONString marshals the args and puts the result into a string
func (a *Args) ToJSONString() string {
	b, err := jsoniter.Marshal(a)
	if err != nil {
		return ""
	}
	return string(b)
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
	emptyInterfaceMsg              = "empty interface name"
	invalidInterfaceMsg            = "invalid interface name"
	invalidQueryTypeMsg            = "invalid query type"
	invalidFormatMsg               = "unknown format"
	invalidNumResults              = "invalid number of result rows"
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
	var (
		err      error
		errModel = &DetailError{
			ErrorModel: huma.ErrorModel{
				Title:  http.StatusText(http.StatusUnprocessableEntity),
				Status: http.StatusUnprocessableEntity,
				Detail: "query preparation failed",
			},
		}
	)

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
		errMsg := err.Error()
		var p *types.ParseError
		if errors.As(err, &p) {
			errMsg = "\n" + p.Pretty()
		}
		// collect error
		errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
			Message:  fmt.Sprintf("%s: %s", invalidQueryTypeMsg, errMsg),
			Location: "body.query",
			Value:    a.Query,
		})
	}

	// verify config format
	_, verifies := permittedFormats[a.Format]
	if !verifies {
		// collect error
		errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
			Message:  fmt.Sprintf("%s: %v", invalidFormatMsg, PermittedFormats()),
			Location: "body.format",
			Value:    a.Format,
		})
	}
	s.Format = a.Format

	// if not already done beforehand, enforce defaults for args
	if a.SortBy == "" {
		a.SortBy = "packets"
	}

	// assign sort order and direction
	s.SortBy, verifies = permittedSortBy[a.SortBy]
	if !verifies {
		// collect error
		errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
			Message:  fmt.Sprintf("%s: %v", invalidSortByMsg, PermittedFormats()),
			Location: "body.sort_by",
			Value:    a.Format,
		})
	}

	// set and validate the interfaces
	if a.Ifaces == "" {
		// collect error
		errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
			Message:  emptyInterfaceMsg,
			Location: "body.ifaces",
			Value:    a.Ifaces,
		})
	}
	s.Ifaces, err = types.ValidateIfaceNames(a.Ifaces)
	if err != nil {
		// collect error
		errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
			Message:  fmt.Sprintf("%s: %s", invalidInterfaceMsg, err),
			Location: "body.ifaces",
			Value:    a.Ifaces,
		})
	}

	// insert iface attribute here in case multiple interfaces where specified and the
	// interface column was not added as an attribute
	if (len(s.Ifaces) > 1 || strings.Contains(a.Ifaces, types.AnySelector)) &&
		!strings.Contains(a.Query, types.IfaceName) {
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
	var timeRangeDetails []*huma.ErrorDetail
	s.First, s.Last, timeRangeDetails = ParseTimeRangeCollectErrors(a.First, a.Last)
	if len(timeRangeDetails) > 0 {
		errModel.Errors = append(errModel.Errors, timeRangeDetails...)
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
			// collect error
			errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
				Message:  fmt.Sprintf("DNS check failed: %s", err),
				Location: "body.dns_resolution.enabled",
				Value:    a.Ifaces,
			})
		}
		if 0 >= s.DNSResolution.Timeout {
			// collect error
			errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
				Message:  invalidDNSResolutionTimeoutMsg,
				Location: "body.dns_resolution.timeout",
				Value:    s.DNSResolution.Timeout,
			})
		}
		if !(0 < s.DNSResolution.MaxRows) {
			// collect error
			errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
				Message:  invalidDNSResolutionTimeoutMsg,
				Location: "body.dns_resolution.max_rows",
				Value:    s.DNSResolution.MaxRows,
			})
		}
	}

	// sanitize conditional if one was provided
	s.Condition = conditions.SanitizeUserInput(a.Condition)

	// build condition tree to check if there is a syntax error before starting processing
	_, _, parseErr := node.ParseAndInstrument(s.Condition, s.DNSResolution.Timeout)
	if parseErr != nil {
		errMsg := parseErr.Error()
		var p *types.ParseError
		if errors.As(parseErr, &p) {
			errMsg = "\n" + p.Pretty()
		}
		// collect error
		errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
			Message:  fmt.Sprintf("%s: %s", invalidConditionMsg, errMsg),
			Location: "body.condition",
			Value:    s.Condition,
		})
	}

	// if we got here, the condition can definitely be tokenized. This makes sure the canonical
	// form of the condition is stored
	tokens, _ := conditions.Tokenize(s.Condition)
	s.Condition = strings.Join(tokens, " ")

	// check memory flag
	if !(0 < a.MaxMemPct && a.MaxMemPct <= 100) {
		// collect error
		errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
			Message:  fmt.Sprintf("%s: must be in (0, 100]", invalidMaxMemPctMsg),
			Location: "body.max_mem_pct",
			Value:    a.MaxMemPct,
		})
	}
	s.MaxMemPct = a.MaxMemPct

	// check limits flag
	if a.NumResults <= 0 {
		// collect error
		errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
			Message:  fmt.Sprintf("%s: must be > 0", invalidNumResults),
			Location: "body.num_results",
			Value:    a.NumResults,
		})
	}
	s.NumResults = a.NumResults

	// check for consistent use of the live flag
	if s.Live && s.Last != types.MaxTime.Unix() {
		// collect error
		errModel.Errors = append(errModel.Errors, &huma.ErrorDetail{
			Message:  fmt.Sprintf("%s: last timestamp unsupported", invalidLiveQueryMsg),
			Location: "live",
			Value:    s.Last,
		})
	}

	// fan-out query results in case multiple writers were supplied
	writers = append(writers, a.outputs...)
	if len(writers) > 0 {
		s.Output = io.MultiWriter(writers...)
	}

	if len(errModel.Errors) > 0 {
		return s, errModel
	}

	return s, nil
}
