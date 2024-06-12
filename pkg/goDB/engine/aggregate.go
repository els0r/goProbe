package engine

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/els0r/goProbe/pkg/types/workload"
	"github.com/els0r/telemetry/logging"
)

type aggregateResult struct {
	aggregatedMaps hashmap.NamedAggFlowMapWithMetadata
	totals         types.Counters
	err            error
}

var numProcessingUnits = runtime.NumCPU()

type internalError int

// enumeration of processing errors
const (
	errorNoResults internalError = iota + 1
	errorMemoryBreach
	errorInternalProcessing
	errorMismatchingHosts
)

// Error implements the error interface for query processing errors
func (i internalError) Error() string {
	switch i {
	case errorMemoryBreach:
		return "memory limit exceeded"
	case errorInternalProcessing:
		return "internal error during query processing"
	}
	return fmt.Sprintf("(!(internalError: %d))", i)
}

func logWorkloadStats(logger *logging.L, msg string, stats *workload.Stats) {
	if stats == nil {
		return
	}
	logger.With("stats", stats).Info(msg)
}

// receive maps on mapChan until mapChan gets closed.
// Then send aggregation result over resultChan.
// If an error occurs, aggregate may return prematurely.
// Closes resultChan on termination.
func (qr *QueryRunner) aggregate(ctx context.Context, mapChan <-chan hashmap.AggFlowMapWithMetadata, ifaces []string, isLowMem bool) chan aggregateResult {
	// create channel that returns the final aggregate result
	resultChan := make(chan aggregateResult, 1)
	logger := logging.FromContext(ctx)

	go func() {
		defer close(resultChan)

		var (
			totals types.Counters
			nAgg   = make(map[string]int)

			// Since we know that the source maps retrieved over the channel are not
			// changed anymore we can re-use the memory allocated for the keys in them by
			// using them for the aggregate map
			finalMaps = hashmap.NewNamedAggFlowMapWithMetadata(ifaces)
		)

		tLastStatsUpdate := time.Now()
		for item := range mapChan {
			if item.IsNil() || item.Interface == "" {
				resultChan <- aggregateResult{err: errorInternalProcessing}
				return
			}

			finalMap := finalMaps[item.Interface]
			finalMap.Stats.Add(item.Stats)

			// the processing stats have been processed. Skip to next item in case there's no flow data to process. This
			// is relevant for cases where no flow records are return as a result of conditions not matching
			if item.Len() == 0 {
				continue
			}

			// Merge the item into the final map for this interface
			finalMap.Merge(item)
			nAgg[item.Interface] = nAgg[item.Interface] + 1

			// keep-alive updating of queries
			if qr.keepAlive > 0 {
				if time.Since(tLastStatsUpdate) > qr.keepAlive {
					tLastStatsUpdate = time.Now()
					logWorkloadStats(logger.With("iface", item.Interface), "processing stats update", finalMap.Stats)
				}
			}

			// Cleanup the now unused item / map
			if isLowMem {
				item.Clear()
			} else {
				item.ClearFast()
			}
		}

		// Push the final result
		if finalMaps.Len() == 0 {
			resultChan <- aggregateResult{}
			return
		}

		resultChan <- aggregateResult{
			aggregatedMaps: finalMaps,
			totals:         totals,
		}
	}()

	return resultChan
}
