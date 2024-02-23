package shellformat

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPackageConsistency(t *testing.T) {
	require.Equal(t, len(allFormats)-1, int(math.Log2(float64(maxFormat))))
}

func TestGenerateEscapeSequences(t *testing.T) {
	type testCase struct {
		input    Format
		expected string
	}

	for _, tc := range []testCase{
		{
			input:    0,
			expected: "",
		},
		{
			input:    Bold,
			expected: EscapeSeqBold,
		},
		{
			input:    Bold | Red,
			expected: EscapeSeqBold + EscapeSeqRed,
		},
		// Note: It is debatable if supporting multiple colors is reasonable, but it should be general
		// IMHO and it would require very explicit checks to handle all these special cases
		{
			input:    Bold | Red | White,
			expected: EscapeSeqBold + EscapeSeqRed + EscapeSeqWhite,
		},
	} {
		require.Equal(t, tc.expected, tc.input.genEscapeSeq())
	}
}

func TestOutput(t *testing.T) {
	type testCase struct {
		format Format
		input  string
		a      []any

		expected string
	}

	// Override isNoColorTerm to allow running the tests anywhere
	isNoColorTerm = false

	now := time.Now()
	for _, tc := range []testCase{
		{
			format:   0,
			input:    "",
			a:        []any{},
			expected: "",
		},
		{
			format:   Bold,
			input:    "BoldTest",
			a:        []any{},
			expected: EscapeSeqBold + "BoldTest" + EscapeSeqReset,
		},
		{
			format:   Bold | Green,
			input:    "This number (%d) and this timestamp (%v) are bold green",
			a:        []any{42, now},
			expected: EscapeSeqBold + EscapeSeqGreen + fmt.Sprintf("This number (%d) and this timestamp (%v) are bold green", 42, now) + EscapeSeqReset,
		},
	} {
		require.Equal(t, tc.expected, Fmt(tc.format, tc.input, tc.a...))
	}
}
