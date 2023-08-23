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

	// StartedAt denotes the time when the capture was started
	StartedAt time.Time `json:"started_at"`

	// Received denotes the number of packets received
	Received uint64 `json:"received"`

	// ReceivedTotal denotes the number of packets received since the capture was started
	ReceivedTotal uint64 `json:"received_total"`

	// Processed denotes the number of packets processed by the capture
	Processed uint64 `json:"processed"`

	// ProcessedTotal denotes the number of packets processed since the capture was started
	ProcessedTotal uint64 `json:"processed_total"`

	// Dropped denotes the number of packets dropped
	Dropped uint64 `json:"dropped"`

	// DroppedTotal denotes the number of packets dropped since the capture was started
	DroppedTotal uint64 `json:"dropped_total"`

	// ParsingErrors denotes all packet parsing errors / failures encountered
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
