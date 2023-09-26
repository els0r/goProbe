package results

import (
	"errors"
	"fmt"
	"io"
	"net/netip"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
)

var (
	// ErrorNoResults denotes that no results were returned from a query
	ErrorNoResults = errors.New("query returned no results")
)

// Result bundles the data rows returned and the query meta information
type Result struct {
	Hostname string `json:"hostname,omitempty"` // Hostname: from which the result originated

	Status        Status        `json:"status"`         // Status: the overall status of the result
	HostsStatuses HostsStatuses `json:"hosts_statuses"` // HostsStatuses: the status of all hosts queried

	Summary Summary `json:"summary"` // Summary: the total traffic volume and packets observed over the queried range and the interfaces that were queried
	Query   Query   `json:"query"`   // Query: the kind of query that was run
	Rows    Rows    `json:"rows"`    // Rows: the data rows returned

	// err is the error encountered when fetching result
	err error `json:"-"`
}

// SetErr will set the error in the status and add it to the hosts statuses
func (hs HostsStatuses) SetErr(host string, err error) {
	if hs == nil {
		return
	}
	hs[host] = Status{
		Code:    types.StatusError,
		Message: err.Error(),
	}
}

// SetErr will set the error in the result and add it to the hosts statuses
// for the current hostname
func (r *Result) SetErr(err error) {
	r.err = err
	r.HostsStatuses.SetErr(r.Hostname, err)
}

// Err returns the error in case the result carries one
func (r *Result) Err() error {
	return r.err
}

// Query stores the kind of query that was run
type Query struct {
	Attributes []string `json:"attributes"`          // Attributes: the attributes that were queried. Example: [sip dip dport proto]
	Condition  string   `json:"condition,omitempty"` // Condition: the condition that was provided. Example: port=80 && proto=TCP
}

// TimeRange describes the interval for which data is queried and presented
type TimeRange struct {
	// First is the start of the interval
	First time.Time `json:"time_first"`
	// Last is the end of the interval
	Last time.Time `json:"time_last"`
}

// Summary stores the total traffic volume and packets observed over the
// queried range and the interfaces that were queried
type Summary struct {
	Interfaces []string `json:"interfaces"` // Interfaces: the interfaces that were queried
	TimeRange
	Totals  types.Counters `json:"totals"`  // Totals: the total traffic volume and packets observed over the queried range
	Timings Timings        `json:"timings"` // Timings: query runtime fields
	Hits    Hits           `json:"hits"`    // Hits: how many flow records were returned in total and how many are returned in Rows
}

// Status denotes the overall status of the result
type Status struct {
	Code    types.Status `json:"code"`              // Code: the status code
	Message string       `json:"message,omitempty"` // Message: an optional message
}

// Timings summarizes query runtimes
type Timings struct {
	QueryStart         time.Time     `json:"query_start"`          // QueryStart: the time when the query started
	QueryDuration      time.Duration `json:"query_duration_ns"`    // QueryDuration: the time it took to run the query in nanoseconds
	ResolutionDuration time.Duration `json:"resolution,omitempty"` // ResolutionDuration: the time it took to resolve all IPs in nanoseconds
}

// Hits stores how many flow records were returned in total and how many are
// returned in Rows
type Hits struct {
	Displayed int `json:"displayed"` // Displayed: how manyflow records were returned in Rows that are displayed
	Total     int `json:"total"`     // Total: how many flow records matching the condition were found in total
}

// Row is a human-readable, aggregatable representation of goDB's data
type Row struct {
	// Labels are the partition Attributes
	Labels Labels `json:"labels,omitempty"`

	// Attributes which can be grouped by
	Attributes Attributes `json:"attributes"`

	// Counters for bytes/packets
	Counters types.Counters `json:"counters"`
}

// Labels hold labels by which the goDB database is partitioned
type Labels struct {
	Timestamp time.Time `json:"timestamp,omitempty"` // Timestamp: the timestamp of the 5-minute interval storing the flow record
	Iface     string    `json:"iface,omitempty"`     // Iface: the interface on which the flow was observed
	Hostname  string    `json:"host,omitempty"`      // Hostname: the hostname of the host on which the flow was observed
	HostID    string    `json:"host_id,omitempty"`   // HostID: the host id of the host on which the flow was observed
}

// Attributes are traffic attributes by which the goDB can be aggregated
type Attributes struct {
	SrcIP   netip.Addr `json:"sip,omitempty"`   // SrcIP: the source IP address
	DstIP   netip.Addr `json:"dip,omitempty"`   // DstIP: the destination IP address
	IPProto uint8      `json:"proto,omitempty"` // IPProto: the IP protocol number
	DstPort uint16     `json:"dport,omitempty"` // DstPort: the destination port
}

// New instantiates a new result
func New() *Result {
	return &Result{
		Status: Status{
			Code: types.StatusOK,
		},
		HostsStatuses: make(HostsStatuses),
	}
}

// Start prepares the beginning of the result
func (r *Result) Start() {
	r.Summary = Summary{
		Timings: Timings{
			QueryStart: time.Now(),
		},
	}
	r.HostsStatuses = make(HostsStatuses)
}

// End prepares the end of the result
func (r *Result) End() {
	r.Summary.Timings.QueryDuration = time.Since(r.Summary.Timings.QueryStart)
	if len(r.Rows) == 0 {
		r.Status = Status{
			Code:    types.StatusEmpty,
			Message: ErrorNoResults.Error(),
		}
	}
	sort.Strings(r.Summary.Interfaces)
}

// HostsStatuses captures the query status for every host queried
type HostsStatuses map[string]Status

// Print adds the status of all hosts to the output / writer
func (hs HostsStatuses) Print(w io.Writer) error {
	var hosts []struct {
		host string
		Status
	}

	var ok, empty, withError int
	for host, status := range hs {
		switch status.Code {
		case types.StatusOK:
			ok++
		case types.StatusEmpty:
			empty++
		case types.StatusError:
			withError++
		}
		hosts = append(hosts, struct {
			host string
			Status
		}{host: host, Status: status})
	}
	sort.SliceStable(hosts, func(i, j int) bool {
		return hosts[i].host < hosts[j].host
	})

	fmt.Fprintf(w, "Hosts: %d ok / %d empty / %d error\n\n", ok, empty, withError)

	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', tabwriter.AlignRight)

	sep := "\t"

	header := []string{"#", "host", "status", "message"}
	fmtStr := sep + strings.Join([]string{"%d", "%s", "%s", "%s"}, sep) + sep + "\n"

	fmt.Fprintln(tw, sep+strings.Join(header, sep)+sep)
	// fmt.Fprintln(tw, sep+strings.Repeat(sep, len(header))+sep)

	for i, host := range hosts {
		fmt.Fprintf(tw, fmtStr, i+1, host.host, host.Code, host.Message)
	}

	return tw.Flush()
}

// String prints the statistics
func (h Hits) String() string {
	return fmt.Sprintf("{total: %d, displayed: %d}", h.Total, h.Displayed)
}

// String prints a single result
func (r *Row) String() string {
	return fmt.Sprintf("%s; %s; %s", r.Labels.String(), r.Attributes.String(), r.Counters.String())
}

// Less returns wether the row r sorts before r2
func (r *Row) Less(r2 *Row) bool {
	if r.Attributes == r2.Attributes {
		return r.Labels.Less(r2.Labels)
	}
	return r.Attributes.Less(r2.Attributes)
}

// ExtendedRow is a human-readable, aggregatable representation of goProbe's active
// flow data
type ExtendedRow struct {
	// Labels are the partition Attributes
	Labels Labels `json:"l,omitempty"`

	// Attributes store ExtendedAttributes which can be grouped by
	Attributes ExtendedAttributes `json:"a"`

	// Counters for bytes/packets
	Counters types.Counters `json:"c"`
}

// MarshalJSON implements the json.Marshaler interface. It makes sure
// that empty timestamps don't show up in the json output
func (l Labels) MarshalJSON() ([]byte, error) {
	var aux = struct {
		// TODO: this is expensive. Check how to get rid of re-assigning
		// values in order to properly treat empties
		Timestamp *time.Time `json:"timestamp,omitempty"`
		Iface     string     `json:"iface,omitempty"`
		Hostname  string     `json:"host,omitempty"`
		HostID    string     `json:"host_id,omitempty"`
	}{
		nil,
		l.Iface,
		l.Hostname,
		l.HostID,
	}
	if !l.Timestamp.IsZero() {
		aux.Timestamp = &l.Timestamp
	}
	return jsoniter.Marshal(aux)
}

// String prints all result labels
func (l Labels) String() string {
	return fmt.Sprintf("ts=%s iface=%s hostname=%s hostID=%s",
		l.Timestamp,
		l.Iface,
		l.Hostname,
		l.HostID,
	)
}

// Less returns wether the set of labels l sorts before l2
func (l Labels) Less(l2 Labels) bool {
	if l.Timestamp != l2.Timestamp {
		return l.Timestamp.Before(l2.Timestamp)
	}

	// Since sorting is about human-readable information this ignores the hostID, assuming
	// that for sorting identical hostnames imply the same host
	if l.Hostname != l2.Hostname {
		return l.Hostname < l2.Hostname
	}

	return l.Iface < l2.Iface
}

// ExtendedAttributes includes the source port. It is meant to be used if (and only if)
// the source port is still available (such as in the flow log)
type ExtendedAttributes struct {
	SrcPort uint16 `json:"sport,omitempty"` // SrcPort: the source port
	Attributes
}

// MarshalJSON marshals an attribute set into a JSON byte slice
func (a Attributes) MarshalJSON() ([]byte, error) {
	var aux = struct {
		// TODO: this is expensive. Check how to get rid of re-assigning
		// values in order to properly treat empties
		SrcIP   *netip.Addr `json:"sip,omitempty"`
		DstIP   *netip.Addr `json:"dip,omitempty"`
		IPProto uint8       `json:"proto,omitempty"`
		DstPort uint16      `json:"dport,omitempty"`
	}{
		IPProto: a.IPProto,
		DstPort: a.DstPort,
	}
	if a.SrcIP.IsValid() {
		aux.SrcIP = &a.SrcIP
	}
	if a.DstIP.IsValid() {
		aux.DstIP = &a.DstIP
	}
	return jsoniter.Marshal(aux)
}

// String prints all result attributes
func (a Attributes) String() string {
	return fmt.Sprintf("sip=%s dip=%s proto=%d dport=%d",
		a.SrcIP.String(),
		a.DstIP.String(),
		a.IPProto,
		a.DstPort,
	)
}

// Less returns wether the set of attributes a sorts before a2
func (a Attributes) Less(a2 Attributes) bool {
	if a.SrcIP != a2.SrcIP {
		return a.SrcIP.Less(a2.SrcIP)
	}
	if a.DstIP != a2.DstIP {
		return a.DstIP.Less(a2.DstIP)
	}
	if a.IPProto != a2.IPProto {
		return a.IPProto < a2.IPProto
	}
	return a.DstPort < a2.DstPort
}

// Rows is a list of results
type Rows []Row

// MergeableAttributes bundles all fields of a Result by which aggregation/merging is possible
type MergeableAttributes struct {
	Labels
	Attributes
}

// RowsMap is an aggregated representation of a Rows list
type RowsMap map[MergeableAttributes]types.Counters

// MergeRows aggregates Rows by use of the RowsMap rm, which is modified
// in the process
func (rm RowsMap) MergeRows(r Rows) (merged int) {
	for _, res := range r {
		counters, exists := rm[MergeableAttributes{res.Labels, res.Attributes}]
		if exists {
			rm[MergeableAttributes{res.Labels, res.Attributes}] = counters.Add(res.Counters)
			merged++
		} else {
			rm[MergeableAttributes{res.Labels, res.Attributes}] = res.Counters
		}
	}
	return
}

// MergeRowsMap aggregates all results of om and stores them in rm
func (rm RowsMap) MergeRowsMap(om RowsMap) (merged int) {
	for oma, oc := range om {
		counters, exists := rm[oma]
		if exists {
			rm[oma] = counters.Add(oc)
			merged++
		} else {
			rm[oma] = oc
		}
	}
	return
}

// ToRowsSorted uses the available sorting functions for Rows to produce
// a sorted Rows list from rm
func (rm RowsMap) ToRowsSorted(order by) Rows {
	r := rm.ToRows()
	order.Sort(r)
	return r
}

// ToRows produces a flat list of Rows from rm. Due to randomized map access,
// this list will _not_ be in any particular order. Use ToRowsSorted if you rely
// on order instead
func (rm RowsMap) ToRows() Rows {
	var r = make(Rows, len(rm))
	if len(rm) == 0 {
		return r
	}
	i := 0
	for ma, c := range rm {
		r[i] = Row{
			Labels:     ma.Labels,
			Attributes: ma.Attributes,
			Counters:   c,
		}
		i++
	}
	return r
}
