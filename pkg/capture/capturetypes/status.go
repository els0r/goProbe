package capturetypes

import (
	"time"

	"github.com/els0r/goProbe/pkg/types/hashmap"
)

// TaggedAggFlowMap represents an aggregated
// flow map tagged with Stats and an
// an interface name.
//
// Used by Manager to return the results of
// RotateAll() and Update().
type TaggedAggFlowMap struct {
	Map   *hashmap.AggFlowMap
	Stats CaptureStats `json:"stats,omitempty"`
	Iface string       `json:"iface"`
}

// InterfaceStats stores the statistics for each interface
type InterfaceStats map[string]CaptureStats

// CaptureStats stores the capture stores its statistics
type CaptureStats struct {
	StartedAt      time.Time `json:"started_at"`      // StartedAt: denotes the time when the capture was started. Example: "2021-01-01T00:00:00Z"
	Received       uint64    `json:"received"`        // Received: denotes the number of packets received. Example: 69
	ReceivedTotal  uint64    `json:"received_total"`  // ReceivedTotal: denotes the number of packets received since the capture was started. Example: 69000
	Processed      uint64    `json:"processed"`       // Processed: denotes the number of packets processed by the capture. Example: 70
	ProcessedTotal uint64    `json:"processed_total"` // ProcessedTotal denotes the number of packets processed since the capture was started. Example: 70000
	Dropped        uint64    `json:"dropped"`         // Dropped: denotes the number of packets dropped. Example: 3
	DroppedTotal   uint64    `json:"dropped_total"`   // DroppedTotal: denotes the number of packets dropped since the capture was started. Example: 20

	// ParsingErrors: denotes all packet parsing errors / failures encountered
	// Example: [23, 0]
	ParsingErrors ParsingErrTracker `json:"parsing_errors,omitempty"`
}

// AddStats is a convenience method to total capture stats. This is relevant in the scope of
// adding statistics from the two directions. The result of the addition is written back
// to a to reduce allocations
func AddStats(a, b *CaptureStats) {
	if a == nil || b == nil {
		return
	}
	a.Received += b.Received
	a.Dropped += b.Dropped
}

// SubStats is a convenience method to total capture stats. This is relevant in the scope of
// subtracting statistics from the two directions. The result of the subtraction is written back
// to a to reduce allocations
func SubStats(a, b *CaptureStats) {
	if a == nil || b == nil {
		return
	}
	a.Received -= b.Received
	a.Dropped -= b.Dropped
}
