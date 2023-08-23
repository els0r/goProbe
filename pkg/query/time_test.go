package query

import (
	"fmt"
	"testing"

	"github.com/els0r/goProbe/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestParseTimestamp(t *testing.T) {
	var tests = []string{
		// special cases
		"-100000d",
		"-100000h",
		"-100000m",
		"-100000s",
		"-23d:4h:3m",
		"-23d:4h:8m:3s",
		"-23d4h8m3s",
		"1674492267",
		"2006-01-02T15:04:05-07:00",             // RFC3339 test
		"Mon Jan 23 11:31:04 2023",              // ANSIC test
		fmt.Sprintf("%d", types.MaxTime.Unix()), // Maximum supported time
	}
	tests = append(tests, timeFormats[2:]...)

	for _, tStr := range tests {
		t.Run(tStr, func(t *testing.T) {
			tstamp, err := ParseTimeArgument(tStr)

			assert.Nil(t, err, "unexpected error: %v", err)
			assert.NotEqual(t, tstamp, 0, "expected non-zero timestamp")
		})
	}
}
