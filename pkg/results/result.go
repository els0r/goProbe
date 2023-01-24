package results

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/els0r/goProbe/pkg/types"
)

// Result bundles the data rows returned and the query meta information
type Result struct {
	Status  types.Status `json:"status"`
	Summary Summary      `json:"summary"`
	Query   Query        `json:"query"`
	Rows    Rows         `json:"rows"`
}

// Query stores the kind of query that was run
type Query struct {
	Attributes []string `json:"attributes"`
	Condition  string   `json:"condition"`
}

// Summary stores the total traffic volume and packets observed over the
// queried range and the interfaces that were queried
type Summary struct {
	Interfaces []string  `json:"interfaces"`
	TimeFirst  time.Time `json:"time_first"`
	TimeLast   time.Time `json:"time_last"`
	Totals     Counters  `json:"totals"`
	Timings    Timings   `json:"timings"`
	Hits       Hits      `json:"hits"`
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
	Labels Labels `json:"l"`

	// Attributes which can be grouped by
	Attributes Attributes `json:"a"`

	// Counters
	Counters Counters `json:"c"`
}

// Labels hold labels by which the goDB database is partitioned
type Labels struct {
	Timestamp *time.Time `json:"timestamp,omitempty"`
	Iface     string     `json:"iface,omitempty"`
	Hostname  string     `json:"host,omitempty"`
	HostID    uint       `json:"host_id,omitempty"`
}

// Attributes are traffic attributes by which the goDB can be aggregated
type Attributes struct {
	SrcIP   netip.Addr `json:"sip,omitempty"`
	DstIP   netip.Addr `json:"dip,omitempty"`
	IPProto uint8      `json:"proto,omitempty"`
	DstPort uint16     `json:"dport,omitempty"`
}

// Counters are the traffic's byte and packet counters
type Counters struct {
	BytesReceived   uint64 `json:"br,omitempty"`
	BytesSent       uint64 `json:"bs,omitempty"`
	PacketsReceived uint64 `json:"pr,omitempty"`
	PacketsSent     uint64 `json:"ps,omitempty"`
}

// SumPackets sums the packet received and sent directions
func (c *Counters) SumPackets() uint64 {
	return c.PacketsReceived + c.PacketsSent
}

// SumBytes sums the bytes received and sent directions
func (c *Counters) SumBytes() uint64 {
	return c.BytesReceived + c.BytesSent
}

// String prints a single result
func (r Row) String() string {
	return fmt.Sprintf("%s; %s; %s", r.Labels.String(), r.Attributes.String(), r.Counters.String())
}

// String prints all result labels
func (l Labels) String() string {
	return fmt.Sprintf("ts=%s iface=%s hostname=%s hostID=%d",
		l.Timestamp,
		l.Iface,
		l.Hostname,
		l.HostID,
	)
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

// String prints all result counters
func (c Counters) String() string {
	return fmt.Sprintf("bytes: sent=%d received=%d; packets: sent=%d received=%d",
		c.BytesSent,
		c.BytesReceived,
		c.PacketsSent,
		c.PacketsReceived,
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
type RowsMap map[MergeableAttributes]*Counters

// MergeRows aggregates Rows by use of the RowsMap rm, which is modified
// in the process
func (rm RowsMap) MergeRows(r Rows) {
	for _, res := range r {
		counters, exists := rm[MergeableAttributes{res.Labels, res.Attributes}]
		if exists {
			counters.BytesReceived += res.Counters.BytesReceived
			counters.BytesSent += res.Counters.BytesSent
			counters.PacketsReceived += res.Counters.PacketsReceived
			counters.PacketsSent += res.Counters.PacketsSent
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
			counters.BytesReceived += oc.BytesReceived
			counters.BytesSent += oc.BytesSent
			counters.PacketsReceived += oc.PacketsReceived
			counters.PacketsSent += oc.PacketsSent
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
