package bitpack

import (
	"fmt"
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
		{
			input:  []uint64{0, 1, intPow(2, 63)},
			output: []byte{0x8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x80},
		},
	}

	for _, c := range testTable {

		// Test packing
		buf := Pack(c.input)
		assert.Equal(t, c.output, buf)

		// Test unpacking / round-trip
		orig := Unpack(buf)
		assert.Equal(t, c.input, orig)

		// Test extraction of all individual values
		expectedNeededBytes := c.output[0]
		for i := 0; i < len(c.input); i++ {
			assert.Equal(t, c.input[i], unpackTable[expectedNeededBytes]((buf[i*int(expectedNeededBytes)+1 : i*int(expectedNeededBytes)+1+int(expectedNeededBytes)])))
		}

		for i := 0; i < len(c.input); i++ {
			assert.Equal(t, c.input[i], Uint64At(buf, i, int(expectedNeededBytes)))
		}

		// Test extraction of number of elements
		assert.Equal(t, Len(buf), len(c.input))
	}
}
func TestAllByteWidths(t *testing.T) {
	for i := 0; i < 64; i += 8 {
		t.Run(fmt.Sprintf("%d_bytes", i/8+1), func(t *testing.T) {
			input := []uint64{intPow(2, uint64(i))}

			// Test packing
			buf := Pack(input)

			// Test unpacking / round-trip
			orig := Unpack(buf)
			assert.Equal(t, input, orig)

			assert.Equal(t, Len(buf), len(input))
		})
	}
}

func TestFlipCases(t *testing.T) {
	for nBytes := 1; nBytes <= 8; nBytes++ {
		val := intPow(2, 8*uint64(nBytes)) - 1
		buf := Pack([]uint64{val, val - 1})

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

func BenchmarkEncode(b *testing.B) {

	for nBytes := 1; nBytes <= 8; nBytes++ {
		b.Run(fmt.Sprintf("%d bytes", nBytes), func(b *testing.B) {
			var input []uint64
			for i := 1; i < 512; i++ {
				input = append(input, intPow(2, uint64(8*nBytes-1)))
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(input) * 8))

			//Warmup
			for i := 0; i < 1000000; i++ {
				Pack(input)
			}
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				Pack(input)
			}
		})
	}

	b.Run("mixed", func(b *testing.B) {
		var input []uint64
		for i := 1; i < 512; i++ {
			input = append(input, intPow(2, 7))
			input = append(input, intPow(2, 15))
			input = append(input, intPow(2, 23))
			input = append(input, intPow(2, 31))
			input = append(input, intPow(2, 39))
			input = append(input, intPow(2, 47))
			input = append(input, intPow(2, 55))
			input = append(input, intPow(2, 63))
		}

		b.ReportAllocs()
		b.SetBytes(int64(len(input) * 8))

		//Warmup
		for i := 0; i < 1000000; i++ {
			Pack(input)
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			Pack(input)
		}
	})
}

func BenchmarkDecodeAsBlock(b *testing.B) {

	for nBytes := 1; nBytes <= 8; nBytes++ {
		b.Run(fmt.Sprintf("%d bytes", nBytes), func(b *testing.B) {
			var input []uint64
			for i := 1; i < 512; i++ {
				input = append(input, intPow(2, uint64(8*nBytes-1)))
			}

			buf := Pack(input)

			b.ReportAllocs()
			b.SetBytes(int64(len(input) * 8))

			//Warmup
			for i := 0; i < 1000000; i++ {
				Unpack(buf)
			}
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				Unpack(buf)
			}
		})
	}

	b.Run("mixed", func(b *testing.B) {
		var input []uint64
		for i := 1; i <= 64; i++ {
			input = append(input, intPow(2, 7))
			input = append(input, intPow(2, 15))
			input = append(input, intPow(2, 23))
			input = append(input, intPow(2, 31))
			input = append(input, intPow(2, 39))
			input = append(input, intPow(2, 47))
			input = append(input, intPow(2, 55))
			input = append(input, intPow(2, 63))
		}

		buf := Pack(input)

		b.ReportAllocs()
		b.SetBytes(int64(len(input) * 8))

		//Warmup
		for i := 0; i < 1000000; i++ {
			Unpack(buf)
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			Unpack(buf)
		}
	})
}

func BenchmarkDecodeIndividually(b *testing.B) {

	for nBytes := 1; nBytes <= 8; nBytes++ {
		b.Run(fmt.Sprintf("%d bytes", nBytes), func(b *testing.B) {
			var input []uint64
			for i := 1; i <= 512; i++ {
				input = append(input, intPow(2, uint64(8*nBytes-1)))
			}

			buf := Pack(input)
			neededBytes := ByteWidth(buf)

			b.ReportAllocs()
			b.SetBytes(int64(len(input) * 8))

			//Warmup
			for i := 0; i < 1000000; i++ {
				for j := 0; j < 512; j++ {
					v := Uint64At(buf, j, neededBytes)
					_ = v
				}
			}
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				for j := 0; j < 512; j++ {
					v := Uint64At(buf, j, neededBytes)
					_ = v
				}
			}
		})
	}

	b.Run("mixed", func(b *testing.B) {
		var input []uint64
		for i := 1; i <= 64; i++ {
			input = append(input, intPow(2, 7))
			input = append(input, intPow(2, 15))
			input = append(input, intPow(2, 23))
			input = append(input, intPow(2, 31))
			input = append(input, intPow(2, 39))
			input = append(input, intPow(2, 47))
			input = append(input, intPow(2, 55))
			input = append(input, intPow(2, 63))
		}

		buf := Pack(input)
		neededBytes := ByteWidth(buf)

		b.ReportAllocs()
		b.SetBytes(int64(len(input) * 8))

		//Warmup
		for i := 0; i < 1000000; i++ {
			for j := 0; j < 512; j++ {
				v := Uint64At(buf, j, neededBytes)
				_ = v
			}
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			for j := 0; j < 512; j++ {
				v := Uint64At(buf, j, neededBytes)
				_ = v
			}
		}
	})
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
