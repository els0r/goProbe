package results

import (
	"fmt"
	"testing"
	"time"

	"github.com/els0r/goProbe/v4/pkg/types"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestMerge(t *testing.T) {

	// t0 := time.Now()

	var tests = []struct {
		inMap    RowsMap
		input    Rows
		expected Rows
	}{}

	for i, test := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			test.inMap.MergeRows(test.input)

			out := test.inMap.ToRowsSorted(By(SortTime, types.DirectionBoth, true))

			assert.Equal(t, test.expected, out)

			b, _ := jsoniter.MarshalIndent(out, "", "  ")
			fmt.Println(string(b))
		})
	}
}

func TestToRowsTo(t *testing.T) {
	tests := []struct {
		name              string
		rowsMapInput      RowsMap
		inputRows         Rows
		expectedRowCount  int
		expectedNonNil    bool
		expectedCapacity  int
		expectedClearance bool // whether input slice was cleared
	}{
		{
			name:             "empty RowsMap with nil rows",
			rowsMapInput:     RowsMap{},
			inputRows:        nil,
			expectedRowCount: 0,
			expectedNonNil:   false,
		},
		{
			name:             "empty RowsMap with empty rows",
			rowsMapInput:     RowsMap{},
			inputRows:        make(Rows, 0),
			expectedRowCount: 0,
			expectedNonNil:   true,
		},
		{
			name:             "empty RowsMap with non-empty rows",
			rowsMapInput:     RowsMap{},
			inputRows:        make(Rows, 0, 10),
			expectedRowCount: 0,
			expectedNonNil:   true,
			expectedCapacity: 10,
		},
		{
			name:             "non-empty RowsMap with nil rows",
			rowsMapInput:     createTestRowsMap(1),
			inputRows:        nil,
			expectedRowCount: 1,
			expectedNonNil:   true,
		},
		{
			name:             "non-empty RowsMap with insufficient capacity",
			rowsMapInput:     createTestRowsMap(5),
			inputRows:        make(Rows, 0, 2),
			expectedRowCount: 5,
			expectedNonNil:   true,
		},
		{
			name:             "non-empty RowsMap with sufficient capacity",
			rowsMapInput:     createTestRowsMap(3),
			inputRows:        make(Rows, 0, 10),
			expectedRowCount: 3,
			expectedNonNil:   true,
			expectedCapacity: 10,
		},
		{
			name:              "non-empty RowsMap with non-empty rows that gets cleared",
			rowsMapInput:      createTestRowsMap(2),
			inputRows:         make(Rows, 5, 10),
			expectedRowCount:  2,
			expectedNonNil:    true,
			expectedCapacity:  10,
			expectedClearance: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.rowsMapInput.ToRowsTo(test.inputRows)

			assert.Equal(t, test.expectedRowCount, len(result), "row count mismatch")

			if test.expectedNonNil {
				assert.NotNil(t, result, "expected non-nil result")
			} else {
				assert.Nil(t, result, "expected nil result")
			}

			if test.expectedCapacity > 0 {
				assert.GreaterOrEqual(t, cap(result), test.expectedCapacity, "capacity should be at least expected capacity")
			}

			// for the case where we had a pre-filled slice with sufficient capacity,
			// verify that the slice was actually cleared (length becomes 0 before appending)
			if test.expectedClearance && len(test.inputRows) > 0 {
				assert.Equal(t, test.expectedRowCount, len(result), "slice should have been cleared")
				assert.GreaterOrEqual(t, cap(result), test.expectedRowCount, "capacity should not shrink")
			}

			// Verify all rows in the result are valid Row objects
			for _, row := range result {
				assert.NotNil(t, row)
			}
		})
	}
}

// createTestRowsMap creates a RowsMap with n rows for testing
func createTestRowsMap(n int) RowsMap {
	rm := make(RowsMap)
	now := time.Now()
	for i := 0; i < n; i++ {
		ma := MergeableAttributes{
			Labels: Labels{
				Timestamp: now,
				Iface:     fmt.Sprintf("eth%d", i),
				Hostname:  "testhost",
			},
			Attributes: Attributes{
				DstPort: uint16(80 + i),
			},
		}
		rm[ma] = types.Counters{
			BytesRcvd: uint64(1000 * (i + 1)),
			BytesSent: uint64(2000 * (i + 1)),
		}
	}
	return rm
}
