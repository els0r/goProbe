package query

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/els0r/goProbe/pkg/goDB"
)

const (
	// Variables for manual garbage collection calls
	goGCInterval = 5 * time.Second
	goGCLimit    = 6291456 // Limit for GC call, in bytes
)

const (
	errorNoResults = "query returned no results"
)

type aggregateResult struct {
	aggregatedMap map[goDB.ExtraKey]goDB.Val
	totals        Counts
	err           error
}

// Counts is a convenience wrapper around the summed counters
type Counts struct {
	PktsRcvd, PktsSent   uint64
	BytesRcvd, BytesSent uint64
}

// receive maps on mapChan until mapChan gets closed.
// Then send aggregation result over resultChan.
// If an error occurs, aggregate may return prematurely.
// Closes resultChan on termination.
func aggregate(mapChan <-chan map[goDB.ExtraKey]goDB.Val, resultChan chan<- aggregateResult) {
	defer close(resultChan)

	var finalMap = make(map[goDB.ExtraKey]goDB.Val)
	var totals Counts

	// Temporary goDB.Val because map values cannot be updated in-place
	var tempVal goDB.Val
	var exists bool

	// Create global MemStats object for tracking of memory consumption
	m := runtime.MemStats{}
	lastGC := time.Now()

	for item := range mapChan {
		if item == nil {
			resultChan <- aggregateResult{
				err: fmt.Errorf("Error during daily DB processing. Check syslog/messages for more information"),
			}
			return
		}
		for k, v := range item {
			totals.BytesRcvd += v.NBytesRcvd
			totals.BytesSent += v.NBytesSent
			totals.PktsRcvd += v.NPktsRcvd
			totals.PktsSent += v.NPktsSent

			if tempVal, exists = finalMap[k]; exists {
				tempVal.NBytesRcvd += v.NBytesRcvd
				tempVal.NBytesSent += v.NBytesSent
				tempVal.NPktsRcvd += v.NPktsRcvd
				tempVal.NPktsSent += v.NPktsSent

				finalMap[k] = tempVal
			} else {
				finalMap[k] = v
			}
		}

		item = nil

		// Conditionally call a manual garbage collection and memory release if the current heap allocation
		// is above goGCLimit and more than goGCInterval seconds have passed
		runtime.ReadMemStats(&m)
		if m.Sys-m.HeapReleased > goGCLimit && time.Since(lastGC) > goGCInterval {
			runtime.GC()
			debug.FreeOSMemory()
			lastGC = time.Now()
		}
	}

	if len(finalMap) == 0 {
		resultChan <- aggregateResult{
			err: fmt.Errorf(errorNoResults),
		}
		return
	}

	resultChan <- aggregateResult{
		aggregatedMap: finalMap,
		totals:        totals,
	}
	return
}
