package bitpack

import "math/bits"

// Pack compresses a slice of uint64 values into a byte slice using the minimal
// possible number of bytes to represent all values in the input slice.
// The first byte of the output is reserved to hold the byte with for decompression
func Pack(data []uint64, buf []byte) []byte {
	neededBytes := getNeededBytes(data)

	buf = append(buf, neededBytes)
	for _, v := range data {
		buf = appendUvarint(buf, v, neededBytes)
	}

	return buf
}

// Unpack decompresses a previously compressed data slice into the original slice of
// uint64 values
func Unpack(buf []byte) []uint64 {
	neededBytes := buf[0]
	res := make([]uint64, 0)

	for i := 1; i < len(buf); i += int(neededBytes) {
		var val uint64
		for j := 0; j < int(neededBytes); j++ {
			val |= uint64(buf[i+j]) << (j * 8)
		}
		res = append(res, val)
	}

	return res
}

// Uint64 returns the decoded singular value for the provided (sub)slice, assuming that
// the length of the slice represents the byte width of the encoding
// TODO: Might need some optimizations to get rid of the loop
func Uint64(buf []byte) (res uint64) {
	for j := 0; j < len(buf); j++ {
		res |= uint64(buf[j]) << (j * 8)
	}

	return res
}

// Uint64At returns the decoded singular value from the provided slice at a given index from the
// original slice
// TODO: Maybe hand over width in call to avoid repeated readout?
func Uint64At(buf []byte, at int) (res uint64) {
	neededBytes := int(buf[0])
	return Uint64(buf[neededBytes*at+1 : neededBytes*at+1+neededBytes])
}

// Len returns the number of encoded elements in the compressed buffer / byte slice
func Len(buf []byte) int {
	if len(buf) == 0 {
		return 0
	}
	neededBytes := buf[0]

	return (len(buf) - 1) / int(neededBytes)
}

func appendUvarint(buf []byte, x uint64, byteWidth byte) []byte {
	for i := 0; i < int(byteWidth); i++ {
		buf = append(buf, byte(x))
		x >>= 8
	}

	return buf
}

func getNeededBytes(data []uint64) byte {
	var maxVal uint64
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}

	return neededBytes(maxVal)
}

func neededBytes(val uint64) byte {
	neededBits := bits.Len64(val)
	if neededBits < 1 {
		return 1
	}

	neededBytes := neededBits / 8
	if neededBits%8 == 0 {
		return byte(neededBytes)
	}

	return byte(neededBytes + 1)
}
