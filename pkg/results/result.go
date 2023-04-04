package results

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/els0r/goProbe/pkg/types"
)

var (
	ErrorNoResults = errors.New("query returned no results")
)

// Result bundles the data rows returned and the query meta information
type Result struct {
	Status Status `json:"status"`

	Summary Summary `json:"summary"`
	Query   Query   `json:"query"`
	Rows    Rows    `json:"rows"`
}

// Query stores the kind of query that was run
type Query struct {
	Attributes []string `json:"attributes"`
	Condition  string   `json:"condition,omitempty"`
}

// Summary stores the total traffic volume and packets observed over the
// queried range and the interfaces that were queried
type Summary struct {
	Interfaces []string       `json:"interfaces"`
	Hosts      []string       `json:"hosts"`
	TimeFirst  time.Time      `json:"time_first"`
	TimeLast   time.Time      `json:"time_last"`
	Totals     types.Counters `json:"totals"`
	Timings    Timings        `json:"timings"`
	Hits       Hits           `json:"hits"`
}

type Status struct {
	Code    types.Status `json:"code"`
	Message string       `json:"message,omitempty"`
}

// HostsStatus captures the query status for every host queried
type HostsStatuses map[string]Status

func (hs HostsStatuses) PrintErrorHosts(w io.Writer) {
	var errHosts []struct {
		host string
		Status
	}

	for host, status := range hs {
		if status.Code != types.StatusOK {
			errHosts = append(errHosts, struct {
				host string
				Status
			}{host: host, Status: status})
		}
	}

	if len(errHosts) == 0 {
		return
	}

	sort.SliceStable(errHosts, func(i, j int) bool {
		return errHosts[i].host < errHosts[j].host
	})

	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', tabwriter.AlignRight)

	sep := "\t"

	header := []string{"#", "host", "status", "message"}
	fmtStr := sep + strings.Join([]string{"%d", "%s", "%s", "%s"}, sep) + sep + "\n"

	fmt.Fprintf(w, "Hosts with errors: %d\n\n", len(errHosts))

	fmt.Fprintln(tw, sep+strings.Join(header, sep)+sep)
	fmt.Fprintln(tw, sep+strings.Repeat(sep, len(header))+sep)

	for i, errHost := range errHosts {
		fmt.Fprintf(tw, fmtStr, i+1, errHost.host, errHost.Code, errHost.Message)
	}
	tw.Flush()
}

// Timinigs summarizes query runtimes
type Timings struct {
	QueryStart         time.Time     `json:"query_start"`
	QueryDuration      time.Duration `json:"query_duration"`
	ResolutionDuration time.Duration `json:"resolution,omitempty"`
}

// Hits stores how many flow records were returned in total and how many are
// returned in Rows
type Hits struct {
	Displayed int `json:"displayed"`
	Total     int `json:"total"`
}

// String prints the statistics
func (h Hits) String() string {
	return fmt.Sprintf("{total: %d, displayed: %d}", h.Total, h.Displayed)
}

// Row is a human-readable, aggregatable representation of goDB's data
type Row struct {
	// Partition Attributes
	Labels Labels `json:"l,omitempty"`

	// Attributes which can be grouped by
	Attributes Attributes `json:"a"`

	// Counters
	Counters types.Counters `json:"c"`
}

// Labels hold labels by which the goDB database is partitioned
type Labels struct {
	Timestamp time.Time `json:"timestamp,omitempty"`
	Iface     string    `json:"iface,omitempty"`
	Hostname  string    `json:"host,omitempty"`
	HostID    string    `json:"host_id,omitempty"`
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
	return json.Marshal(aux)
}

// Attributes are traffic attributes by which the goDB can be aggregated
type Attributes struct {
	SrcIP   netip.Addr `json:"sip,omitempty"`
	DstIP   netip.Addr `json:"dip,omitempty"`
	IPProto uint8      `json:"proto,omitempty"`
	DstPort uint16     `json:"dport,omitempty"`
}

// String prints a single result
func (r Row) String() string {
	return fmt.Sprintf("%s; %s; %s", r.Labels.String(), r.Attributes.String(), r.Counters.String())
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
	return json.Marshal(aux)
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

// Rows is a list of results
type Rows []Row

// MergeableAttributes bundles all fields of a Result by which aggregation/merging is possible
type MergeableAttributes struct {
	Labels
	Attributes
}

// RowsMap is an aggregated representation of a Rows list
type RowsMap map[MergeableAttributes]*types.Counters

// MergeRows aggregates Rows by use of the RowsMap rm, which is modified
// in the process
func (rm RowsMap) MergeRows(r Rows) {
	for _, res := range r {
		counters, exists := rm[MergeableAttributes{res.Labels, res.Attributes}]
		if exists {
			*counters = counters.Add(res.Counters)
		} else {
			rm[MergeableAttributes{res.Labels, res.Attributes}] = &res.Counters
		}
	}
}

// MergeRowsMap aggregates all results of om and stores them in rm
func (rm RowsMap) MergeRowsMap(om RowsMap) {
	for oma, oc := range om {
		counters, exists := rm[oma]
		if exists {
			*counters = counters.Add(*oc)
		} else {
			rm[oma] = oc
		}
	}
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
			Counters:   *c,
		}
		i++
	}
	return r
}
