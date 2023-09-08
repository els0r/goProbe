package results

import (
	"fmt"
	"testing"

	"github.com/els0r/goProbe/pkg/types"
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
