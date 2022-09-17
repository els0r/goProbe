package query

import (
	"net"
	"time"
)

type Result struct {
	Attributes ResultAttributes `json:"attr"`
	Values     ResultValues     `json:"vals"`
	Metadata   *ResultMetadata  `json:"meta"`
}

type ResultMetadata struct {
	SrcDomain string `json:"src_domain"`
	DstDomain string `json:"dst_domain"`
}

type ResultAttributes struct {
	Hostname  string    `json:"hostname,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Iface     string    `json:"iface,omitempty"`

	SrcIP   *net.IP `json:"sip"`
	DstIP   *net.IP `json:"dip"`
	IPProto *uint8  `json:"proto"`
	DstPort *uint16 `json:"dport"`
}

type ResultValues struct {
	Bytes struct {
		Received *uint64 `json:"rcvd"`
		Sent     *uint64 `json:"sent"`
	} `json:"bytes"`

	Packets struct {
		Received *uint64 `json:"rcvd"`
		Sent     *uint64 `json:"sent"`
	} `json:"packets"`
}

type Results []*Result
