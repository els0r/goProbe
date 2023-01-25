package query

import (
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/results"
)

type aggregateResult struct {
	aggregatedMap map[goDB.ExtraKey]goDB.Val
	totals        results.Counters
	err           error
}

// receive maps on mapChan until mapChan gets closed.
// Then send aggregation result over resultChan.
// If an error occurs, aggregate may return prematurely.
// Closes resultChan on termination.
func aggregate(mapChan <-chan map[goDB.ExtraKey]goDB.Val) chan aggregateResult {

	// create channel that returns the final aggregate result
	resultChan := make(chan aggregateResult, 1)

	go func() {
		defer close(resultChan)

		var finalMap = make(map[goDB.ExtraKey]goDB.Val)
		var totals results.Counters

		// Temporary goDB.Val because map values cannot be updated in-place
		var tempVal goDB.Val
		var exists bool

		for item := range mapChan {
			if item == nil {
				resultChan <- aggregateResult{err: errorInternalProcessing}
				return
			}

			for k, v := range item {
				totals.BytesReceived += v.NBytesRcvd
				totals.BytesSent += v.NBytesSent
				totals.PacketsReceived += v.NPktsRcvd
				totals.PacketsSent += v.NPktsSent

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
		}

		// push the final result
		if len(finalMap) == 0 {
			resultChan <- aggregateResult{}
			return
		}

		resultChan <- aggregateResult{
			aggregatedMap: finalMap,
			totals:        totals,
		}
	}()
	return resultChan
}
