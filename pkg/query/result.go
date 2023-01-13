package query

import (
	"net/netip"
	"time"
)

type Result struct {
	// Partition Attributes
	Labels struct {
		Timestamp time.Time `json:"timestamp,omitempty"`
		Iface     string    `json:"iface,omitempty"`
		Hostname  string    `json:"host,omitempty"`
		HostID    uint      `json:"host_id,omitempty"`
	} `json:"labels"`

	// Attributes which can be grouped by
	Attributes struct {
		SrcIP   netip.Addr `json:"src_ip,omitempty"`
		DstIP   netip.Addr `json:"dst_ip,omitempty"`
		IPProto uint8      `json:"proto,omitempty"`
		DstPort uint16     `json:"dport,omitempty"`
	} `json:"attributes"`

	// Counters
	Counters struct {
		BytesReceived   uint64 `json:"bytes_rcvd,omitempty"`
		BytesSent       uint64 `json:"bytes_sent,omitempty"`
		PacketsReceived uint64 `json:"pkts_rcvd,omitempty"`
		PacketsSent     uint64 `json:"pkts_sent,omitempty"`
	} `json:"counters"`
}
