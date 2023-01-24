package bitpack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	input  []uint64
	output []byte
}

func TestTable(t *testing.T) {
	var testTable = []testCase{
		{
			input:  []uint64{},
			output: []byte{0x1},
		},
		{
			input:  []uint64{0},
			output: []byte{0x1, 0x0},
		},
		{
			input:  []uint64{0, 1},
			output: []byte{0x1, 0x0, 0x1},
		},
	}

	for _, c := range testTable {

		var buf []byte

		// Test packing
		buf = Pack(c.input, buf)
		assert.Equal(t, c.output, buf)

		// Test unpacking / round-trip
		orig := Unpack(buf)
		assert.Equal(t, c.input, orig)

		// Test extraction of all individual values
		expectedNeededBytes := c.output[0]
		for i := 0; i < len(c.input); i++ {
			assert.Equal(t, c.input[i], Uint64(buf[i*int(expectedNeededBytes)+1:i*int(expectedNeededBytes)+1+int(expectedNeededBytes)]))
		}

		// Test extraction of number of elements
		assert.Equal(t, Len(buf), len(c.input))
	}
}

func TestFlipCases(t *testing.T) {
	for nBytes := 1; nBytes <= 8; nBytes++ {
		val := intPow(2, 8*uint64(nBytes)) - 1
		var buf []byte
		buf = Pack([]uint64{val, val - 1}, buf)

		assert.Equal(t, buf[0], byte(nBytes))
		assert.Equal(t, len(buf), 2*nBytes+1)
		for i := 1; i <= nBytes; i++ {
			assert.Equal(t, buf[i], byte(255))
		}
		assert.Equal(t, buf[nBytes+1], byte(254))
		for i := nBytes + 2; i <= nBytes*2; i++ {
			assert.Equal(t, buf[i], byte(255))
		}
	}
}

func intPow(n, m uint64) uint64 {
	if m == 0 {
		return 1
	}
	result := n
	for i := uint64(2); i <= m; i++ {
		result *= n
	}
	return result
}
