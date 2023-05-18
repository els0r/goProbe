package capturetypes

import (
	"fmt"
	"sort"
	"strings"

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

// CaptureStats stores the capture stores its statistics
// TODO: Track errors and similar counters in metrics
type CaptureStats struct {
	Received       int `json:"received"`
	ReceivedTotal  int `json:"received_total"`
	Processed      int `json:"processed"`
	ProcessedTotal int `json:"processed_total"`
	Dropped        int `json:"dropped"`
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
