package capturetypes

import (
	"fmt"
	"sort"
	"strings"
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
// TODO: Track errors and similar counters in metrics
type CaptureStats struct {
	// StartedAt denotes the time when the capture was started
	StartedAt time.Time `json:"started_at"`
	// Received denotes the number of packets received
	Received int `json:"received"`
	// Received denotes the number of packets received since the capture was started
	ReceivedTotal int `json:"received_total"`
	// Processed denotes the number of packets processed by the capture
	Processed int `json:"processed"`
	// Processed denotes the number of packets processed since the capture was started
	ProcessedTotal int `json:"processed_total"`
	// Dropped denotes the number of packets dropped
	Dropped int `json:"dropped"`
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

// ErrorMap stores all encountered pcap errors and their number of occurrence
type ErrorMap map[string]int

// String prints the errors that occurred during capturing
func (e ErrorMap) String() string {
	var errs = make([]string, len(e))

	i := 0
	for err, count := range e {
		errs[i] = fmt.Sprintf("%s (%d)", err, count)
		i++
	}
	sort.Slice(errs, func(i, j int) bool {
		return errs[i] < errs[j]
	})
	return strings.Join(errs, "; ")
}
