// Package results provides result handling and aggregation for goProbe queries,
// including time binning functionality for automatic time resolution scaling.
package results

import (
	"context"
	"sync"
	"time"

	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/telemetry/tracing"
)

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

var rowsMapPool = sync.Pool{
	New: func() any {
		return make(RowsMap)
	},
}

// GetRowsMap retrieves a RowsMap from the pool for reuse
func GetRowsMap() RowsMap {
	return rowsMapPool.Get().(RowsMap)
}

// PutRowsMap clears the map and puts it back into the pool for reuse
func PutRowsMap(m RowsMap) {
	clear(m)
	rowsMapPool.Put(m)
}

// BinTime applies time binning to the result, re-aggregating rows with the same
// binned timestamp and attributes
func (t *TimeBinner) BinTime(ctx context.Context, res *Result) error {
	if res == nil {
		return nil
	}

	if len(res.Rows) == 0 {
		return nil
	}

	_, span := tracing.Start(ctx, "(*TimeBinner).BinTime")
	defer span.End()

	// re-aggregate rows using RowsMap with binned timestamps
	rowsMap := GetRowsMap()
	for _, row := range res.Rows {
		binnedRow := row
		if !row.Labels.Timestamp.IsZero() {
			binnedTS := BinTimestamp(row.Labels.Timestamp.Unix(), t.binSize)
			binnedRow.Labels.Timestamp = time.Unix(binnedTS, 0)
		}

		rowsMap.MergeRow(binnedRow)
	}

	// convert the map back to a sorted slice of rows, re-using the existing rows slice
	res.Rows = rowsMap.ToRowsSortedTo(res.Rows, By(SortTime, types.DirectionSum, true))

	res.Summary.Hits.Total = len(res.Rows)
	res.Summary.Hits.Displayed = len(res.Rows)

	PutRowsMap(rowsMap)
	return nil
}

// CalcTimeBinSize calculates the time bin size for a given duration,
// ensuring that the number of bins does not exceed
func CalcTimeBinSize(resolution, duration time.Duration) time.Duration {
	if duration <= 0 || resolution <= 0 {
		return types.DefaultTimeResolution
	}

	numBlocksPerDay := 24 * time.Hour / resolution

	// calculate the minimum bin size to fit within numBlocksPerDay
	binSize := duration / numBlocksPerDay

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
	return (ts - remainder) + binSizeSeconds
}
