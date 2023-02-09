package bitpack

import (
	"math/bits"
)

// Pack compresses a slice of uint64 values into a byte slice using the minimal
// possible number of bytes to represent all values in the input slice.
// The first byte of the output is reserved to hold the byte with for decompression
func Pack(data []uint64) []byte {
	neededBytes := getNeededBytes(data)

	b := make([]byte, 1+len(data)*neededBytes)
	b[0] = byte(neededBytes)

	b2 := b[1:]
	switch neededBytes {
	case 1:
		packAll1(b2, data)
	case 2:
		packAll2(b2, data)
	case 3:
		packAll3(b2, data)
	case 4:
		packAll4(b2, data)
	case 5:
		packAll5(b2, data)
	case 6:
		packAll6(b2, data)
	case 7:
		packAll7(b2, data)
	default:
		packAll8(b2, data)
	}

	return b
}

func UnpackInto(b []byte, res []uint64) []uint64 {
	neededBytes := int(b[0])
	nElements := (len(b) - 1) / neededBytes

	if cap(res) < nElements {
		res = make([]uint64, nElements, nElements)
	}
	res = res[:nElements]

	b2 := b[1:]
	switch neededBytes {
	case 1:
		unpackAll1(b2, res, nElements)
	case 2:
		unpackAll2(b2, res, nElements)
	case 3:
		unpackAll3(b2, res, nElements)
	case 4:
		unpackAll4(b2, res, nElements)
	case 5:
		unpackAll5(b2, res, nElements)
	case 6:
		unpackAll6(b2, res, nElements)
	case 7:
		unpackAll7(b2, res, nElements)
	default:
		unpackAll8(b2, res, nElements)
	}

	return res
}

// Unpack decompresses a previously compressed data slice into the original slice of
// uint64 values
func Unpack(b []byte) []uint64 {
	neededBytes := int(b[0])
	nElements := (len(b) - 1) / neededBytes
	res := make([]uint64, nElements)

	b2 := b[1:]
	switch neededBytes {
	case 1:
		unpackAll1(b2, res, nElements)
	case 2:
		unpackAll2(b2, res, nElements)
	case 3:
		unpackAll3(b2, res, nElements)
	case 4:
		unpackAll4(b2, res, nElements)
	case 5:
		unpackAll5(b2, res, nElements)
	case 6:
		unpackAll6(b2, res, nElements)
	case 7:
		unpackAll7(b2, res, nElements)
	default:
		unpackAll8(b2, res, nElements)
	}

	return res
}

// Uint64At returns the decoded singular value from the provided slice at a given index from the
// original slice
func Uint64At(b []byte, at int, neededBytes int) uint64 {
	return unpackTable[neededBytes]((b[neededBytes*at+1 : neededBytes*at+1+neededBytes]))
}

// Len returns the number of encoded elements in the compressed bfer / byte slice
func Len(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	return (len(b) - 1) / ByteWidth(b)
}

// ByteWidth returns the amount of bytes used to encode each element in the input
// from the encoded byte slice
func ByteWidth(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	return int(b[0])
}

////////////////////////////////////////////////////////////////////////////////////////

func getNeededBytes(data []uint64) int {
	var maxVal uint64
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}

	return neededBytes(maxVal)
}

func neededBytes(val uint64) int {
	neededBits := bits.Len64(val)
	if neededBits < 1 {
		return 1
	}

	neededBytes := neededBits / 8
	if neededBits%8 == 0 {
		return neededBytes
	}

	return neededBytes + 1
}

func pack1(b []byte, x uint64) {
	_ = b[0] // bounds check hint to compiler; see golang.org/issue/14808
	b[0] = byte(x)
}
func pack2(b []byte, x uint64) {
	_ = b[1] // bounds check hint to compiler; see golang.org/issue/14808
	b[0] = byte(x)
	b[1] = byte(x >> 8)
}
func pack3(b []byte, x uint64) {
	_ = b[2] // bounds check hint to compiler; see golang.org/issue/14808
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
}
func pack4(b []byte, x uint64) {
	_ = b[3] // bounds check hint to compiler; see golang.org/issue/14808
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
}
func pack5(b []byte, x uint64) {
	_ = b[4] // bounds check hint to compiler; see golang.org/issue/14808
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
}
func pack6(b []byte, x uint64) {
	_ = b[5] // bounds check hint to compiler; see golang.org/issue/14808
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
	b[5] = byte(x >> 40)
}
func pack7(b []byte, x uint64) {
	_ = b[6] // bounds check hint to compiler; see golang.org/issue/14808
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
	b[5] = byte(x >> 40)
	b[6] = byte(x >> 48)
}
func pack8(b []byte, x uint64) {
	_ = b[7] // bounds check hint to compiler; see golang.org/issue/14808
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
	b[5] = byte(x >> 40)
	b[6] = byte(x >> 48)
	b[7] = byte(x >> 56)
}

func packAll1(b []byte, data []uint64) {
	for i := 0; i < len(data); i++ {
		pack1(b[i:i+1], data[i])
	}
}
func packAll2(b []byte, data []uint64) {
	for i := 0; i < len(data); i++ {
		pack2(b[i*2:i*2+2], data[i])
	}
}
func packAll3(b []byte, data []uint64) {
	for i := 0; i < len(data); i++ {
		pack3(b[i*3:i*3+3], data[i])
	}
}
func packAll4(b []byte, data []uint64) {
	for i := 0; i < len(data); i++ {
		pack4(b[i*4:i*4+4], data[i])
	}
}
func packAll5(b []byte, data []uint64) {
	for i := 0; i < len(data); i++ {
		pack5(b[i*5:i*5+5], data[i])
	}
}
func packAll6(b []byte, data []uint64) {
	for i := 0; i < len(data); i++ {
		pack6(b[i*6:i*6+6], data[i])
	}
}
func packAll7(b []byte, data []uint64) {
	for i := 0; i < len(data); i++ {
		pack7(b[i*7:i*7+7], data[i])
	}
}
func packAll8(b []byte, data []uint64) {
	for i := 0; i < len(data); i++ {
		pack8(b[i*8:i*8+8], data[i])
	}
}

func unpack1(b []byte) uint64 {
	_ = b[0] // bounds check hint to compiler; see golang.org/issue/14808
	return uint64(b[0])
}
func unpack2(b []byte) uint64 {
	_ = b[1] // bounds check hint to compiler; see golang.org/issue/14808
	return uint64(b[0]) | uint64(b[1])<<8
}
func unpack3(b []byte) uint64 {
	_ = b[2] // bounds check hint to compiler; see golang.org/issue/14808
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16
}
func unpack4(b []byte) uint64 {
	_ = b[3] // bounds check hint to compiler; see golang.org/issue/14808
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24
}
func unpack5(b []byte) uint64 {
	_ = b[4] // bounds check hint to compiler; see golang.org/issue/14808
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32
}
func unpack6(b []byte) uint64 {
	_ = b[5] // bounds check hint to compiler; see golang.org/issue/14808
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40
}
func unpack7(b []byte) uint64 {
	_ = b[6] // bounds check hint to compiler; see golang.org/issue/14808
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48
}
func unpack8(b []byte) uint64 {
	_ = b[7] // bounds check hint to compiler; see golang.org/issue/14808
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}

func unpackAll1(b []byte, res []uint64, n int) {
	for i := 0; i < n; i++ {
		res[i] = unpack1(b[i:])
	}
}
func unpackAll2(b []byte, res []uint64, n int) {
	for i := 0; i < n; i++ {
		res[i] = unpack2(b[i*2:])
	}
}
func unpackAll3(b []byte, res []uint64, n int) {
	for i := 0; i < n; i++ {
		res[i] = unpack3(b[i*3:])
	}
}
func unpackAll4(b []byte, res []uint64, n int) {
	for i := 0; i < n; i++ {
		res[i] = unpack4(b[i*4:])
	}
}
func unpackAll5(b []byte, res []uint64, n int) {
	for i := 0; i < n; i++ {
		res[i] = unpack5(b[i*5:])
	}
}
func unpackAll6(b []byte, res []uint64, n int) {
	for i := 0; i < n; i++ {
		res[i] = unpack6(b[i*6:])
	}
}
func unpackAll7(b []byte, res []uint64, n int) {
	for i := 0; i < n; i++ {
		res[i] = unpack7(b[i*7:])
	}
}
func unpackAll8(b []byte, res []uint64, n int) {
	for i := 0; i < n; i++ {
		res[i] = unpack8(b[i*8:])
	}
}

var unpackTable = [9]func(b []byte) uint64{
	0x00: nil, // Should never happen (and panic)
	0x01: unpack1,
	0x02: unpack2,
	0x03: unpack3,
	0x04: unpack4,
	0x05: unpack5,
	0x06: unpack6,
	0x07: unpack7,
	0x08: unpack8,
}
