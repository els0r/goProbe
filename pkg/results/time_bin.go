// Package results provides result handling and aggregation for goProbe queries,
// including time binning functionality for automatic time resolution scaling.
package results

import (
	"context"
	"time"

	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/telemetry/tracing"
)

// AutoMode is the string value indicating automatic bin size calculation
const AutoMode = "auto"

// NumBlocksPerDay defines the target maximum number of blocks per query
// This corresponds to 24 hours of 5-minute blocks (24 * 60 / 5 = 288)
const NumBlocksPerDay = 288

// PostProcessor is a function that post-processes a query result
type PostProcessor func(context.Context, *Result) (*Result, error)

// TimeBinner applies time binning to aggregate results to a coarser time resolution
type TimeBinner struct {
	queryRange time.Duration
	binSize    time.Duration
}

// NewTimeBinner creates a new TimeBinner with an explicit bin size
func NewTimeBinner(queryRange, binSize time.Duration) *TimeBinner {
	return &TimeBinner{
		queryRange: queryRange,
		binSize:    binSize,
	}
}

// BinTime applies time binning to the result, re-aggregating rows with the same
// binned timestamp and attributes
func (t *TimeBinner) BinTime(ctx context.Context, res *Result) (*Result, error) {
	if res == nil {
		return res, nil
	}

	// If no rows, nothing to bin
	if len(res.Rows) == 0 {
		return res, nil
	}

	_, span := tracing.Start(ctx, "(*TimeBinner).BinTime")
	defer span.End()

	// Re-aggregate rows using RowsMap with binned timestamps
	rowsMap := make(RowsMap)
	for _, row := range res.Rows {
		// Create a copy of the row with binned timestamp
		binnedRow := row
		if !row.Labels.Timestamp.IsZero() {
			binnedTS := BinTimestamp(row.Labels.Timestamp.Unix(), t.binSize)
			binnedRow.Labels.Timestamp = time.Unix(binnedTS, 0)
		}

		// Merge into the map (rows with identical labels+attributes will aggregate)
		rowsMap.MergeRows(Rows{binnedRow})
	}

	// Convert back to sorted rows. Sort by time since binning is a time-based operation
	res.Rows = rowsMap.ToRowsSorted(By(SortTime, types.DirectionSum, true))
	res.Summary.Hits.Total = len(res.Rows)
	res.Summary.Hits.Displayed = len(res.Rows)

	return res, nil
}

// CalcTimeBinSize calculates the time bin size for a given duration,
// ensuring that the number of bins does not exceed NumBlocksPerDay.
// The result is always rounded up to the nearest 5-minute increment.
func CalcTimeBinSize(duration time.Duration) time.Duration {
	if duration <= 0 {
		return 5 * time.Minute
	}

	// Calculate the minimum bin size to fit within NumBlocksPerDay
	binSize := duration / NumBlocksPerDay

	// Round up to the nearest 5-minute increment
	fiveMinutes := 5 * time.Minute
	if binSize%fiveMinutes != 0 {
		binSize = ((binSize / fiveMinutes) + 1) * fiveMinutes
	}

	return binSize
}

// BinTimestamp bins a timestamp to the end of its bin period, accounting for the
// interval that each timestamp represents. A timestamp t represents flows in the interval
// [t - DefaultBucketSize, t]. The binning uses ceiling division to find the bin end
// that the timestamp belongs to.
func BinTimestamp(ts int64, binSize time.Duration) int64 {
	if binSize <= 0 {
		return ts
	}

	binSizeSeconds := int64(binSize.Seconds())
	if binSizeSeconds <= 0 {
		return ts
	}

	remainder := ts % binSizeSeconds
	if remainder == 0 {
		return ts
	}

	// reduce to the nearest lower multiple of binSize, then add binSize to get the ceiling.
	// e.g. 14:31 -> 14:30 at bin size = 15m ==> 14:30 + 15m = 14:45
	return (ts - (ts % binSizeSeconds)) + binSizeSeconds
}
