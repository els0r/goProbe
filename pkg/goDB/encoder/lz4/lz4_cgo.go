//go:build cgo && !goprobe_noliblz4
// +build cgo,!goprobe_noliblz4

package lz4

/*
#cgo linux CFLAGS: -O3
#cgo linux LDFLAGS: -O3 -llz4
#cgo darwin,amd64 LDFLAGS: -O3 -llz4
#cgo darwin,arm64 LDFLAGS: -O3 -llz4
#include "lz4.h"
#include "lz4hc.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"io"
	"unsafe"
)

// Close will close the encoder and release potentially allocated resources
func (e *Encoder) Close() error {
	return nil
}

// Compress compresses the input data and writes it to dst
func (e *Encoder) Compress(data, buf []byte, dst io.Writer) (n int, err error) {

	// Handle output slice size
	dstCapacity := int(C.LZ4_compressBound(
		C.int(len(data)),
	))
	if cap(buf) < dstCapacity {
		buf = make([]byte, 0, 2*dstCapacity)
	}
	buf = buf[:dstCapacity]

	// Handle special case of empty input data
	var dataPtr unsafe.Pointer
	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}

	// Compress data
	compLen := int(C.LZ4_compress_HC(
		(*C.char)(dataPtr),
		(*C.char)(unsafe.Pointer(&buf[0])),
		C.int(len(data)),
		C.int(dstCapacity),
		C.int(e.level)),
	)
	if compLen <= 0 {
		return n, fmt.Errorf("lz4: compression failed: (errno %d)", compLen)
	}

	// Perform sanity check whether the computed worst case has been exceeded in C call
	if len(buf) < compLen {
		return n, errors.New("lz4: buffer size mismatch for compressed data")
	}

	// If provided, write output to the writer
	if dst != nil {
		if n, err = dst.Write(buf[:compLen]); err != nil {
			return n, err
		}
	}

	return n, nil
}

// Decompress runs LZ4 decompression on "in" read from "src" and writes it to "out"
func (e *Encoder) Decompress(in, out []byte, src io.Reader) (int, error) {
	var nBytesConsumed int

	// Read compressed source data
	nBytesConsumed, err := src.Read(in)
	if err != nil {
		return 0, err
	}
	if nBytesConsumed != len(in) {
		return 0, errors.New("lz4: incorrect number of bytes read from data source")
	}

	// Handle special case of empty output data
	var outPtr unsafe.Pointer
	if len(out) > 0 {
		outPtr = unsafe.Pointer(&out[0])
	}

	// Decompress data
	decompLen := int(C.LZ4_decompress_safe(
		(*C.char)(unsafe.Pointer(&in[0])),
		(*C.char)(outPtr),
		C.int(len(in)),
		C.int(len(out))))
	if decompLen < 0 {
		return 0, fmt.Errorf("lz4: decompression failed: (errno %d)", decompLen)
	}

	return decompLen, nil
}
