package capturetypes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/els0r/goProbe/pkg/types/hashmap"
)

// State enumerates the activity states of a capture
// type State byte

// const (
// 	// StateInitializing means that the capture is setting up
// 	StateInitializing State = iota + 1
// 	// StateCapturing means that the capture is actively capturing packets
// 	StateCapturing
// 	// StateClosing means that the capture is fully terminating and it's held resources are
// 	// cleaned up
// 	StateClosing
// 	// StateError means that the capture has hit the error threshold on the interface (set by ErrorThreshold)
// 	StateError
// )

// func (cs State) String() string {
// 	switch cs {
// 	case StateInitializing:
// 		return "initializing"
// 	case StateCapturing:
// 		return "capturing"
// 	case StateClosing:
// 		return "closing"
// 	case StateError:
// 		return "inError"
// 	default:
// 		return "unknown"
// 	}
// }

// PacketStats stores the packet statistics of the capture
// type PacketStats struct {
// 	*CaptureStats
// 	PacketsCapturedOverall int
// }

// // InterfaceStatus stores both the capture's state and statistics
// type InterfaceStatus struct {
// 	// State       State       `json:"state"`
// 	PacketStats PacketStats `json:"packet_stats"`
// }

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
