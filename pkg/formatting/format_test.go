package formatting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCountHumanized(t *testing.T) {
	var tests = []struct {
		input    uint64
		expected string
	}{
		{0, "0.00  "},
		{1, "1.00  "},
		{100000, "100.00 k"},
		{238327428, "238.33 M"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.expected, func(t *testing.T) {
			actual := Count(test.input)
			require.Equal(t, test.expected, actual)
		})
	}
}

func TestSize(t *testing.T) {
	var tests = []struct {
		input    uint64
		expected string
	}{
		{0, "0.00  B"},
		{231, "231.00  B"},
		{2338231, "2.23 MB"},
		{28319384728, "26.37 GB"},
		{2832828383338231, "2.52 PB"},
		{2832828383338238231, "2.46 EB"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.expected, func(t *testing.T) {
			actual := Size(test.input)
			require.Equal(t, test.expected, actual)
		})
	}
}

func TestDuration(t *testing.T) {
	var tests = []struct {
		input    time.Duration
		expected string
	}{
		{0, "0s"},
		{1 * time.Millisecond, "1ms"},
		{1 * time.Second, "1s"},
		{1*time.Second + 232*time.Millisecond, "1.232s"},
		{1*time.Minute + 3*time.Second, "1m3s"},
		{1*time.Hour + 3*time.Minute + 3*time.Second, "1h3m3s"},
		{25*time.Hour + 3*time.Minute + 3*time.Second, "1d1h3m3s"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.expected, func(t *testing.T) {
			actual := Duration(test.input)
			require.Equal(t, test.expected, actual)
		})
	}
}
