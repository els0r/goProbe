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

	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/goProbe/v4/pkg/types/workload"
	jsoniter "github.com/json-iterator/go"
)

var (
	// ErrorNoResults denotes that no results were returned from a query
	ErrorNoResults = errors.New("query returned no results")

	// ErrorDataMissing denotes that there was no data to be queried
	ErrorDataMissing = errors.New("no data available for the specified interface(s) / time range (maybe goProbe was not running)")
)

// Result bundles the data rows returned and the query meta information
type Result struct {
	// Hostname: from which the result originated
	Hostname string `json:"hostname,omitempty" doc:"Hostname from which the result originated" example:"hostA"`

	// Status: the overall status of the result
	Status Status `json:"status" doc:"Status of the result"`
	// HostsStatuses: the status of all hosts queried
	HostsStatuses HostsStatuses `json:"hosts_statuses" doc:"Statuses of all hosts queried"`

	// Summary: the total traffic volume and packets observed over the queried range and the interfaces that were queried
	Summary Summary `json:"summary" doc:"Traffic totals and query statistics"`
	// Query: the kind of query that was run
	Query Query `json:"query" doc:"Query which was run"`
	// Rows: the data rows returned
	Rows Rows `json:"rows" doc:"Data rows returned"`

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
	// Attributes: the attributes that were queried
	Attributes []string `json:"attributes" doc:"Attributes which were queried" example:"sip,dip,dport"`
	// Condition: the condition that was provided
	Condition string `json:"condition,omitempty" doc:"Condition which was provided" example:"port=80 && proto=TCP"`
}

// TimeRange describes the interval for which data is queried and presented
type TimeRange struct {
	// First is the start of the interval
	First time.Time `json:"time_first" doc:"Start of the queried interval" example:"2020-08-12T09:47:00+02:00"`
	// Last is the end of the interval
	Last time.Time `json:"time_last" doc:"End of the queried interval" example:"2024-04-12T09:47:00+02:00"`
}

// ResultsRange returns the duration of the time range covered by the result. This MUST be strictly smaller, or equal to
// the query range.
//
// Example: a single flow found in a 7d query range, will result in a 5m results range
func (t TimeRange) ResultsRange() time.Duration {
	return t.Last.Sub(t.First)
}

// Summary stores the total traffic volume and packets observed over the queried range and the interfaces that were queried
type Summary struct {
	// Interfaces: the interfaces that were queried
	Interfaces Interfaces `json:"interfaces" doc:"Interfaces which were queried" example:"eth0,eth1"`
	TimeRange
	// Totals: the total traffic volume and packets observed over the queried range
	Totals types.Counters `json:"totals" doc:"Total traffic volume and packets observed over the queried time range"`
	// Timings: query runtime fields
	Timings Timings `json:"timings" doc:"Query runtime fields"`
	// Hits: how many flow records were returned in total and how many are returned in Rows
	Hits Hits `json:"hits" doc:"Flow records returned in total and records present in rows"`
	// DataAvailable: Was there any data available on disk or from a live query at all
	DataAvailable bool `json:"data_available" doc:"Was there any data available to query at all"`
	// Stats tracks interactions with the underlying DB data
	Stats *workload.Stats `json:"stats,omitempty" doc:"Stats tracks interactions with the underlying DB data"`
}

// Interfaces collects all interface names
type Interfaces []string

// Status denotes the overall status of the result
type Status struct {
	Code    types.Status `json:"code" doc:"Status code" enum:"empty,error,missing_data,ok" example:"empty"`         // Code: the status code
	Message string       `json:"message,omitempty" doc:"Optional status description" example:"no results returned"` // Message: an optional message
}

// Timings summarizes query runtimes
type Timings struct {
	// QueryStart: the time when the query started
	QueryStart time.Time `json:"query_start" doc:"Query start time"`
	// QueryDuration: the time it took to run the query in nanoseconds
	QueryDuration time.Duration `json:"query_duration_ns" doc:"Query runtime in nanoseconds" example:"235000000"`
	// ResolutionDuration: the time it took to resolve all IPs in nanoseconds
	ResolutionDuration time.Duration `json:"resolution,omitempty" doc:"DNS resolution time for all IPs in nanoseconds" example:"515000000"`
}

// Hits stores how many flow records were returned in total and how many are
// returned in Rows
type Hits struct {
	// Displayed: how many flow records were returned in Rows that are displayed
	Displayed int `json:"displayed" doc:"Number of flow records in Rows displayed/returned" example:"25"`
	// Total: how many flow records matching the condition were found in total
	Total int `json:"total" doc:"Total number of flow records matching the condition" example:"1034"`
}

// Row is a human-readable, aggregatable representation of goDB's data
type Row struct {
	// Labels are the partition Attributes
	Labels Labels `json:"labels,omitempty" doc:"Labels / partitions the row belongs to"`

	// Attributes which can be grouped by
	Attributes Attributes `json:"attributes" doc:"Query attributes by which flows are grouped"`

	// Counters for bytes/packets
	Counters types.Counters `json:"counters" doc:"Flow counters"`
}

// Labels hold labels by which the goDB database is partitioned
type Labels struct {
	// Timestamp: the timestamp of the 5-minute interval storing the flow record
	Timestamp time.Time `json:"timestamp,omitempty" doc:"Timestamp (end) of the 5-minute interval storing the flow record" example:"2024-04-12T03:20:00+02:00"`
	// Iface: the interface on which the flow was observed
	Iface string `json:"iface,omitempty" doc:"Interface on which the flow was observed" example:"eth0"`
	// Hostname: the hostname of the host on which the flow was observed
	Hostname string `json:"host,omitempty" doc:"Hostname of the host on which the flow was observed" example:"hostA"`
	// HostID: the host id of the host on which the flow was observed
	HostID string `json:"host_id,omitempty" doc:"ID of the host on which the flow was observed" example:"123456"`
}

// Attributes are traffic attributes by which the goDB can be aggregated
type Attributes struct {
	SrcIP   netip.Addr `json:"sip,omitempty" doc:"Source IP" example:"10.81.45.1"`   // SrcIP: the source IP address
	DstIP   netip.Addr `json:"dip,omitempty" doc:"Destination IP" example:"8.8.8.8"` // DstIP: the destination IP address
	IPProto uint8      `json:"proto,omitempty" doc:"IP protocol number" example:"6"` // IPProto: the IP protocol number
	DstPort uint16     `json:"dport,omitempty" doc:"Destination port" example:"80"`  // DstPort: the destination port
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
		Stats: &workload.Stats{},
	}
	r.HostsStatuses = make(HostsStatuses)
}

// End prepares the end of the result
func (r *Result) End() {
	r.Summary.Timings.QueryDuration = time.Since(r.Summary.Timings.QueryStart)
	r.Summary.Hits.Displayed = len(r.Rows)
	sort.Strings(r.Summary.Interfaces)
	if len(r.Rows) != 0 {
		return
	}
	if r.Summary.DataAvailable {
		r.Status = Status{
			Code:    types.StatusEmpty,
			Message: ErrorNoResults.Error(),
		}
		return
	}
	r.Status = Status{
		Code:    types.StatusMissingData,
		Message: ErrorDataMissing.Error(),
	}
}

// Summary returns a summary of the interfaces without listing them explicitly
func (is Interfaces) Summary() string {
	return fmt.Sprintf("%d", len(is))
}

// Print prints the sorted interface list in a table with numbered rows
func (is Interfaces) Print(w io.Writer) error {
	sort.SliceStable(is, func(i, j int) bool {
		return is[i] < is[j]
	})

	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', tabwriter.AlignRight)

	sep := "\t"

	header := []string{"#", "iface"}
	fmtStr := sep + strings.Join([]string{"%d", "%s"}, sep) + sep + "\n"

	fmt.Fprintln(tw, sep+strings.Join(header, sep)+sep)

	for i, iface := range is {
		fmt.Fprintf(tw, fmtStr, i+1, iface)
	}

	return tw.Flush()
}

// HostsStatuses captures the query status for every host queried
type HostsStatuses map[string]Status

// Summary returns a summary of the host statuses without going through the individual statuses
func (hs HostsStatuses) Summary() string {
	var ok, empty, withError int
	for _, status := range hs {
		switch status.Code {
		case types.StatusOK:
			ok++
		case types.StatusEmpty:
			empty++
		case types.StatusError:
			withError++
		}
	}
	return fmt.Sprintf("%d hosts: %d ok / %d empty / %d error", len(hs), ok, empty, withError)
}

// HostStatus bundles the Hostname with the Status for that Hostname
type HostStatus struct {
	Hostname string
	Status
}

// GetErrorStatuses returns all hosts which ran into an error during querying
func (hs HostsStatuses) GetErrorStatuses() (errorHosts []HostStatus) {
	for _, host := range hs.getSortedStatuses() {
		if host.Code != types.StatusOK {
			errorHosts = append(errorHosts, host)
		}
	}
	return errorHosts
}

func (hs HostsStatuses) getSortedStatuses() (hosts []HostStatus) {
	for host, status := range hs {
		hosts = append(hosts, HostStatus{Hostname: host, Status: status})
	}
	sort.SliceStable(hosts, func(i, j int) bool {
		return hosts[i].Hostname < hosts[j].Hostname
	})
	return hosts
}

// Print adds the status of all hosts to the output / writer
func (hs HostsStatuses) Print(w io.Writer) error {
	hosts := hs.getSortedStatuses()

	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', tabwriter.AlignRight)

	sep := "\t"

	header := []string{"#", "host", "status", "message"}
	fmtStr := sep + strings.Join([]string{"%d", "%s", "%s", "%s"}, sep) + sep + "\n"

	fmt.Fprintln(tw, sep+strings.Join(header, sep)+sep)

	for i, host := range hosts {
		fmt.Fprintf(tw, fmtStr, i+1, host.Hostname, host.Code, host.Message)
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
	for _, row := range r {
		if rm.MergeRow(row) {
			merged++
		}
	}
	return
}

// MergeRow aggregates a single Row into the RowsMap rm, which is modified in the process.
// It returns whether the row was merged with an existing row (true) or added as a new entry (false)
func (rm RowsMap) MergeRow(row Row) (merged bool) {
	counters, exists := rm[MergeableAttributes{row.Labels, row.Attributes}]
	if exists {
		counters.Add(row.Counters)
		rm[MergeableAttributes{row.Labels, row.Attributes}] = counters
		return true
	}

	rm[MergeableAttributes{row.Labels, row.Attributes}] = row.Counters
	return false
}

// MergeRowsMap aggregates all results of om and stores them in rm
func (rm RowsMap) MergeRowsMap(om RowsMap) (merged int) {
	for oma, oc := range om {
		counters, exists := rm[oma]
		if exists {
			counters.Add(oc)
			rm[oma] = counters
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

// ToRowsSortedTo operates like ToRowsSorted but writes the sorted rows into the provided slice.
//
// If the provided slice is nil or has insufficient capacity, a new slice will be allocated.
func (rm RowsMap) ToRowsSortedTo(rows Rows, order by) Rows {
	rows = rm.ToRowsTo(rows)
	order.Sort(rows)
	return rows
}

// ToRows produces a flat list of Rows from rm. Due to randomized map access,
// this list will _not_ be in any particular order. Use ToRowsSorted if you rely
// on order instead
func (rm RowsMap) ToRows() Rows {
	var r = make(Rows, len(rm))
	return rm.ToRowsTo(r)
}

// ToRowsTo writes a flat list of Rows from rm into the provided slice. Due to randomized map access,
// the slice _will not_ be in any particular order. Use ToRowsSortedTo if you rely on order instead.
//
// If the provided slice is nil or has insufficient capacity, a new slice will be allocated.
// The input slice will be cleared by this function, not retaining any of its previous content.
func (rm RowsMap) ToRowsTo(rows Rows) Rows {
	if len(rm) == 0 {
		return rows
	}
	if rows == nil || cap(rows) < len(rm) {
		rows = make(Rows, 0, len(rm))
	} else {
		rows = rows[:0]
	}
	i := 0
	for ma, c := range rm {
		rows = append(rows, Row{
			Labels:     ma.Labels,
			Attributes: ma.Attributes,
			Counters:   c,
		})
		i++
	}
	return rows
}
