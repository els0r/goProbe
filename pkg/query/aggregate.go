package query

import (
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types/hashmap"
)

type aggregateResult struct {
	aggregatedMap *hashmap.Map
	totals        results.Counters
	err           error
}

// receive maps on mapChan until mapChan gets closed.
// Then send aggregation result over resultChan.
// If an error occurs, aggregate may return prematurely.
// Closes resultChan on termination.
func aggregate(mapChan <-chan hashmap.AggFlowMapWithMetadata) chan aggregateResult {

	// create channel that returns the final aggregate result
	resultChan := make(chan aggregateResult, 1)

	go func() {
		defer close(resultChan)

		// Since we know that the source maps retrieved over the channel are not
		// changed anymore we can re-use the memory allocated for the keys in them by
		// using them for the aggregate map
		var finalMap = hashmap.New().ZeroCopy()
		var totals results.Counters

		for item := range mapChan {
			if item.Map == nil {
				resultChan <- aggregateResult{err: errorInternalProcessing}
				return
			}

			for i := item.Iter(); i.Next(); {
				val := i.Val()
				totals.BytesReceived += val.NBytesRcvd
				totals.BytesSent += val.NBytesSent
				totals.PacketsReceived += val.NPktsRcvd
				totals.PacketsSent += val.NPktsSent

				finalMap.SetOrUpdate(i.Key(),
					val.NBytesRcvd,
					val.NBytesSent,
					val.NPktsRcvd,
					val.NPktsSent,
				)
			}

			item.Map = nil
		}

		// push the final result
		if finalMap.Len() == 0 {
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
