package query

import (
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
)

type aggregateResult struct {
	aggregatedMap hashmap.AggFlowMapWithMetadata
	totals        types.Counters
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
		var finalMap = hashmap.AggFlowMapWithMetadata{
			Map: hashmap.New().ZeroCopy(),
		}

		var (
			totals types.Counters
			nAgg   int
		)

		for item := range mapChan {
			if item.Map == nil {
				resultChan <- aggregateResult{err: errorInternalProcessing}
				return
			}

			// Set the metadata of the final map according to the first element received. For all
			// consecutive elements validate consistency
			// TODO: In theory the Iface attribute is identical for all elements received in an item
			// in this range loop, so we _could_ remove iface handling from the DBWorkManager.go
			// processing entirely and extend the key here during aggregattion.
			if nAgg == 0 {
				finalMap.HostID = item.HostID
				finalMap.Hostname = item.Hostname
			} else {
				if finalMap.HostID != item.HostID || finalMap.Hostname != item.Hostname {
					resultChan <- aggregateResult{err: errorMismatchingHosts}
					return
				}
			}

			for i := item.Iter(); i.Next(); {
				val := i.Val()
				totals = totals.Add(val)

				finalMap.SetOrUpdate(i.Key(),
					val.NBytesRcvd,
					val.NBytesSent,
					val.NPktsRcvd,
					val.NPktsSent,
				)
			}

			nAgg++
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
