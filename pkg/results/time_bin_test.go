package results

import (
	"testing"
	"time"

	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestCalcTimeBinSize(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected time.Duration
	}{
		{
			name:     "below 24 hours",
			duration: 12 * time.Hour,
			expected: 5 * time.Minute, // 12h / 288 = 2.5min, rounded up to 5min
		},
		{
			name:     "exactly 24 hours",
			duration: 24 * time.Hour,
			expected: 5 * time.Minute, // 24h / 288 = 5min, no rounding needed
		},
		{
			name:     "25 hours",
			duration: 25 * time.Hour,
			expected: 10 * time.Minute, // 25h / 288 ≈ 5.2min, rounded up to 10min
		},
		{
			name:     "48 hours",
			duration: 48 * time.Hour,
			expected: 10 * time.Minute, // 48h / 288 = 10min, no rounding needed
		},
		{
			name:     "49 hours",
			duration: 49 * time.Hour,
			expected: 15 * time.Minute, // 49h / 288 ≈ 10.2min, rounded up to 15min
		},
		{
			name:     "72 hours",
			duration: 72 * time.Hour,
			expected: 15 * time.Minute, // 72h / 288 = 15min, no rounding needed
		},
		{
			name:     "73 hours",
			duration: 73 * time.Hour,
			expected: 20 * time.Minute, // 73h / 288 ≈ 15.2min, rounded up to 20min
		},
		{
			name:     "7 days",
			duration: 7 * 24 * time.Hour,
			expected: 35 * time.Minute, // 7d / 288 ≈ 34.7min, rounded up to 35min
		},
		{
			name:     "zero duration",
			duration: 0,
			expected: 5 * time.Minute,
		},
		{
			name:     "negative duration",
			duration: -1 * time.Hour,
			expected: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalcTimeBinSize(tt.duration)
			assert.Equal(t, tt.expected, result, "expected bin size %v, got %v", tt.expected, result)
		})
	}
}

func TestBinTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp int64
		binSize   time.Duration
		expected  int64
	}{
		// Binning uses ceiling division: ceil(ts / binSize) * binSize
		{
			name:      "timestamp at bin boundary",
			timestamp: 600,
			binSize:   300 * time.Second,
			expected:  600,
		},
		{
			name:      "timestamp within bin",
			timestamp: 450,
			binSize:   300 * time.Second,
			expected:  600,
		},
		{
			name:      "timestamp 10 min bin",
			timestamp: 1550,
			binSize:   600 * time.Second,
			expected:  1800,
		},
		{
			name:      "multiple timestamps in same bin",
			timestamp: 1599,
			binSize:   600 * time.Second,
			expected:  1800,
		},
		{
			name:      "timestamp at bin end",
			timestamp: 300,
			binSize:   300 * time.Second,
			expected:  300,
		},
		{
			name:      "zero bin size",
			timestamp: 1500,
			binSize:   0,
			expected:  1500, // Should return timestamp as-is
		},
		{
			name:      "negative bin size",
			timestamp: 1500,
			binSize:   -300 * time.Second,
			expected:  1500, // Should return timestamp as-is
		},
		{
			name:      "15 min bin",
			timestamp: 1234567890,
			binSize:   900 * time.Second, // 15 minutes
			expected:  1234568700,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BinTimestamp(tt.timestamp, tt.binSize)
			assert.Equal(t, tt.expected, result, "expected binned timestamp %d, got %d", tt.expected, result)
		})
	}
}

func TestBinTimestampConsistency(t *testing.T) {
	// Test that multiple timestamps within the same bin have the same binned value
	binSize := 600 * time.Second // 10 minutes
	// All timestamps ceil to same bin end
	ts1 := BinTimestamp(1200, binSize) // ceil(1200/600)*600 = 2*600 = 1200
	ts2 := BinTimestamp(1550, binSize) // ceil(1550/600)*600 = 3*600 = 1800
	ts3 := BinTimestamp(1800, binSize) // ceil(1800/600)*600 = 3*600 = 1800

	assert.Equal(t, ts2, ts3, "timestamps 1550 and 1800 should bin to same value")
	assert.Equal(t, int64(1800), ts2, "both should bin to 1800 (bin end)")
	assert.NotEqual(t, ts1, ts2, "ts1 and ts2 should be in different bins")
}

func TestBinTimestampBoundaries(t *testing.T) {
	// Test bin boundaries with 5-minute bins using ceiling division
	binSize := 300 * time.Second // 5 minutes

	// ts=300: ceil(300/300) = 1, output = 1*300 = 300
	assert.Equal(t, int64(300), BinTimestamp(300, binSize))

	// ts=301: ceil(301/300) = 2, output = 2*300 = 600
	assert.Equal(t, int64(600), BinTimestamp(301, binSize))

	// ts=599: ceil(599/300) = 2, output = 2*300 = 600
	assert.Equal(t, int64(600), BinTimestamp(599, binSize))

	// ts=600: ceil(600/300) = 2, output = 2*300 = 600
	assert.Equal(t, int64(600), BinTimestamp(600, binSize))

	// ts=601: ceil(601/300) = 3, output = 3*300 = 900
	assert.Equal(t, int64(900), BinTimestamp(601, binSize))
}

func TestTimeBinnerBinTime(t *testing.T) {
	// Create a test result with multiple rows at different timestamps
	result := &Result{
		Status: Status{Code: "ok"},
		Rows: Rows{
			{
				Labels: Labels{
					Timestamp: time.Unix(1000, 0),
				},
				Attributes: Attributes{},
				Counters:   types.Counters{BytesRcvd: 100, BytesSent: 200},
			},
			{
				Labels: Labels{
					Timestamp: time.Unix(1150, 0), // Within same 10-min bin as 1000
				},
				Attributes: Attributes{},
				Counters:   types.Counters{BytesRcvd: 50, BytesSent: 75},
			},
			{
				Labels: Labels{
					Timestamp: time.Unix(1600, 0), // Different bin
				},
				Attributes: Attributes{},
				Counters:   types.Counters{BytesRcvd: 30, BytesSent: 45},
			},
		},
	}

	// Create a TimeBinner for 25 hours (should result in 10-minute bins)
	binner := NewTimeBinner(25*time.Hour, 10*time.Minute)

	// Apply binning using ceiling division
	binnedResult, err := binner.BinTime(t.Context(), result)
	assert.NoError(t, err)
	assert.NotNil(t, binnedResult)

	// Should have 2 rows after binning
	// Row 1 (ts=1000): ceil(1000/600)*600 = ceil(1.667)*600 = 2*600 = 1200
	// Row 2 (ts=1150): ceil(1150/600)*600 = ceil(1.917)*600 = 2*600 = 1200 (merges with Row 1)
	// Row 3 (ts=1600): ceil(1600/600)*600 = ceil(2.667)*600 = 3*600 = 1800
	assert.Equal(t, 2, len(binnedResult.Rows), "Expected 2 rows after binning")

	// Check that the first merged row has combined counters
	firstRow := binnedResult.Rows[0]
	assert.Equal(t, time.Unix(1200, 0), firstRow.Labels.Timestamp, "First row should be at 1200 (binned end)")
	assert.Equal(t, uint64(150), firstRow.Counters.BytesRcvd, "BytesRcvd should be sum of 100+50")
	assert.Equal(t, uint64(275), firstRow.Counters.BytesSent, "BytesSent should be sum of 200+75")

	// Check the second row
	secondRow := binnedResult.Rows[1]
	assert.Equal(t, time.Unix(1800, 0), secondRow.Labels.Timestamp, "Second row should be at 1800")
	assert.Equal(t, uint64(30), secondRow.Counters.BytesRcvd, "BytesRcvd should be 30")
	assert.Equal(t, uint64(45), secondRow.Counters.BytesSent, "BytesSent should be 45")
}

func TestTimeBinnerBinTimeWithNilResult(t *testing.T) {
	binner := NewTimeBinner(24*time.Hour, CalcTimeBinSize(24*time.Hour))
	result, err := binner.BinTime(t.Context(), nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestTimeBinnerBinTimeWithEmptyRows(t *testing.T) {
	result := &Result{
		Status: Status{Code: "ok"},
		Rows:   Rows{},
	}

	binner := NewTimeBinner(24*time.Hour, CalcTimeBinSize(24*time.Hour))
	binnedResult, err := binner.BinTime(t.Context(), result)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(binnedResult.Rows))
}
