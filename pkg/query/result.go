package query

import (
	"fmt"
	"net/netip"
	"time"
)

// Result is a human-readable, aggregatable representation of goDB's data
type Result struct {
	// Partition Attributes
	Labels ResultLabels `json:"labels"`

	// Attributes which can be grouped by
	Attributes ResultAttributes `json:"attributes"`

	// Counters
	Counters ResultCounters `json:"counters"`
}

// ResultLabels hold labels by which the goDB database is partitioned
type ResultLabels struct {
	Timestamp time.Time `json:"timestamp,omitempty"`
	Iface     string    `json:"iface,omitempty"`
	Hostname  string    `json:"host,omitempty"`
	HostID    uint      `json:"host_id,omitempty"`
}

// ResultAttributes are traffic attributes by which the goDB can be aggregated
type ResultAttributes struct {
	SrcIP   netip.Addr `json:"src_ip,omitempty"`
	DstIP   netip.Addr `json:"dst_ip,omitempty"`
	IPProto uint8      `json:"proto,omitempty"`
	DstPort uint16     `json:"dport,omitempty"`
}

// ResultCounters are the traffic's byte and packet counters
type ResultCounters struct {
	BytesReceived   uint64 `json:"bytes_rcvd,omitempty"`
	BytesSent       uint64 `json:"bytes_sent,omitempty"`
	PacketsReceived uint64 `json:"pkts_rcvd,omitempty"`
	PacketsSent     uint64 `json:"pkts_sent,omitempty"`
}

// String prints a single result
func (r Result) String() string {
	return fmt.Sprintf("%s; %s; %s", r.Labels.String(), r.Attributes.String(), r.Counters.String())
}

// String prints all result labels
func (l ResultLabels) String() string {
	return fmt.Sprintf("ts=%s iface=%s hostname=%s hostID=%d",
		l.Timestamp,
		l.Iface,
		l.Hostname,
		l.HostID,
	)
}

// String prints all result attributes
func (a ResultAttributes) String() string {
	return fmt.Sprintf("sip=%s dip=%s proto=%d dport=%d",
		a.SrcIP.String(),
		a.DstIP.String(),
		a.IPProto,
		a.DstPort,
	)
}

// String prints all result counters
func (c ResultCounters) String() string {
	return fmt.Sprintf("bytes: sent=%d received=%d; packets: sent=%d received=%d",
		c.BytesSent,
		c.BytesReceived,
		c.PacketsSent,
		c.PacketsReceived,
	)
}

// Results is a list of results
type Results []Result

// MergeableAttributes bundles all fields of a Result by which aggregation/merging is possible
type MergeableAttributes struct {
	ResultLabels
	ResultAttributes
}

// ResultsMap is an aggregated representation of a Results list
type ResultsMap map[MergeableAttributes]*ResultCounters

// MergeResults aggregates Results by use of the ResultsMap rm, which is modified
// in the process
func (rm ResultsMap) MergeResults(r Results) {
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

// MergeResultsMap aggregates all results of om and stores them in rm
func (rm ResultsMap) MergeResultsMap(om ResultsMap) {
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

// ToResultsSorted uses the available sorting functions for Results to produce
// a sorted Results list from rm
func (rm ResultsMap) ToResultsSorted(order by) Results {
	r := rm.ToResults()
	order.Sort(r)
	return r
}

// ToResults produces a flat list of Results from rm. Due to randomized map access,
// this list will _not_ be in any particular order. Use ToResultsSorted if you rely
// on order instead
func (rm ResultsMap) ToResults() Results {
	var r = make([]Result, len(rm))
	if len(rm) == 0 {
		return r
	}
	i := 0
	for ma, c := range rm {
		r[i] = Result{
			Labels:     ma.ResultLabels,
			Attributes: ma.ResultAttributes,
			Counters:   *c,
		}
		i++
	}
	return r
}
