// Package lz4 implements goDB's Encoder interface for lz4 (de-)compression of flow data
package lz4

/*
#cgo linux CFLAGS: -O3
#cgo linux LDFLAGS: -O3 -llz4
#cgo darwin,amd64 LDFLAGS: -O3 -llz4
#cgo darwin,arm64 LDFLAGS: -O3 -llz4
#include <stdint.h>
#include <stdio.h>
#include "lz4frame.h"

LZ4F_dctx* lz4InitDCtx() {

	// Create / initialize decompression context
	LZ4F_dctx* dctx;
	LZ4F_errorCode_t const dctxStatus = LZ4F_createDecompressionContext(&dctx, LZ4F_VERSION);
	if (LZ4F_isError(dctxStatus)) {
		return NULL;
	}

	return dctx;
}

size_t lz4Compress(const char *src, const size_t srcSize, char *dst, const size_t dstSize, const int level) {

	// initialize the frame compression preferences
	// taken from https://github.com/lz4/lz4/blob/dev/examples/frameCompress.c
	const LZ4F_preferences_t prefs = {
		{
			LZ4F_max64KB,
			LZ4F_blockLinked,
			LZ4F_noContentChecksum,
			LZ4F_frame,
			srcSize,
			0,
			LZ4F_noBlockChecksum,
		},
		level,
		1,
		1,
		{ 0, 0, 0 },
	};

	return LZ4F_compressFrame(dst, dstSize, src, srcSize, &prefs);
}

size_t lz4GetCompressBound(const size_t srcSize, const int level) {

	// initialize the frame compression preferences
	// taken from https://github.com/lz4/lz4/blob/dev/examples/frameCompress.c
	const LZ4F_preferences_t prefs = {
		{
			LZ4F_max64KB,
			LZ4F_blockLinked,
			LZ4F_noContentChecksum,
			LZ4F_frame,
			srcSize,
			0,
			LZ4F_noBlockChecksum,
		},
		level,
		1,
		1,
		{ 0, 0, 0 },
	};

	return LZ4F_compressFrameBound(srcSize, &prefs);
}

static const LZ4F_decompressOptions_t decompOpts = {
	1, // pledges that last 64KB decompressed data will remain available unmodified between invocations
	1, // disable checksum calculation and verification to save CPU time. This line is why we even choose to use liblz4 1.9.4
	0, // reserved 0
	0, // reserved 1
};

const char* lz4GetErrorName(const int code) {
	return LZ4F_getErrorName(code);
}

size_t lz4Decompress(uintptr_t dctx, const char *src, const size_t srcSize, char *dst, size_t dstSize) {

	// read from src until there are no more bytes to be read
	const void* srcPtr = (const char*)src;
	const void* srcEnd = (const char*)src + srcSize;
	size_t result = 1;
	while (srcPtr < srcEnd && result != 0) {
		size_t sSize = (const char*)srcEnd - (const char*)srcPtr;

		result = LZ4F_decompress((LZ4F_dctx*)dctx, dst, &dstSize, src, &sSize, &decompOpts);
		if (LZ4F_isError(result)) {
			return result;
		}
		srcPtr = (const char*)srcPtr + sSize;
	}

	// LZ4_decompress writes the amount of decompressed bytes into dSize
	return dstSize;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"io"
	"unsafe"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
)

const defaultCompressionLevel = 6

// Encoder compresses data with the LZ4 algorithm (omitting certain bounds-checks for performance reasons)
type Encoder struct {

	// decompression context
	dCtx *C.LZ4F_dctx

	// compression level
	level int
}

// Option sets additional parameters on the Encoder
type Option func(*Encoder)

// New creates a new LZ4 Encoder that can be used to compress/decompress data
func New(opts ...Option) *Encoder {
	// compression level of 4 is used by default as it offers higher compression speeds than maximum compression,
	// while retaining an agreeable compression ratio
	l := &Encoder{level: defaultCompressionLevel}

	// apply options
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// WithCompressionLevel allows the level to be set to something other than the default 512
func WithCompressionLevel(level int) Option {
	return func(e *Encoder) {
		e.level = level
	}
}

// Type will return the type of encoder
func (e *Encoder) Type() encoders.Type {
	return encoders.EncoderTypeLZ4
}

// Close will close the encoder and release potentially allocated resources
func (e *Encoder) Close() error {
	if e.dCtx != nil {
		if fErr := int(C.LZ4F_freeDecompressionContext(e.dCtx)); fErr < 0 {
			errName := C.GoString(C.lz4GetErrorName(C.int(fErr)))
			return fmt.Errorf("lz4: decompression context release failed: %s", errName)
		}
	}
	return nil
}

// Compress compresses the input data and writes it to dst
func (e *Encoder) Compress(data, buf []byte, dst io.Writer) (n int, err error) {
	dstCapacity := int(C.lz4GetCompressBound(
		C.size_t(len(data)),
		C.int(e.level),
	))

	if cap(buf) < dstCapacity {
		buf = make([]byte, 0, 2*dstCapacity)
	}
	buf = buf[:dstCapacity]

	compLen := int(C.lz4Compress(
		(*C.char)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		(*C.char)(unsafe.Pointer(&buf[0])),
		C.size_t(dstCapacity),
		C.int(e.level)),
	)

	// properly handle error codes from lz4
	if compLen < 0 {
		errName := C.GoString(C.lz4GetErrorName(C.int(compLen)))
		return n, fmt.Errorf("lz4: compression failed: %s (%d)", errName, compLen)
	}

	// sanity check whether the computed worst case has been exceeded in C call
	if len(buf) < compLen {
		return n, errors.New("lz4: buffer size mismatch for compressed data")
	}

	if n, err = dst.Write(buf[:compLen]); err != nil {
		return n, err
	}

	return n, nil
}

// Decompress runs LZ4 decompression on "in" read from "src" and writes it to "out"
func (e *Encoder) Decompress(in, out []byte, src io.Reader) (int, error) {
	var (
		nBytesConsumed int
	)

	// read compressed source data
	nBytesConsumed, err := src.Read(in)
	if err != nil {
		return 0, err
	}
	if nBytesConsumed != len(in) {
		return 0, errors.New("lz4: incorrect number of bytes read from data source")
	}

	// If no decompression context exists, create one
	if e.dCtx == nil {
		if e.dCtx = C.lz4InitDCtx(); e.dCtx == nil {
			return 0, errors.New("lz4: decompression context creation failed")
		}
	}

	// decompress data
	decompLen := int(C.lz4Decompress(
		C.uintptr_t(uintptr(unsafe.Pointer(e.dCtx))),
		(*C.char)(unsafe.Pointer(&in[0])),
		C.size_t(len(in)),
		(*C.char)(unsafe.Pointer(&out[0])),
		C.size_t(len(out))))
	if decompLen < 0 {
		errName := C.GoString(C.lz4GetErrorName(C.int(decompLen)))
		return 0, fmt.Errorf("lz4: decompression failed: %s (%d)", errName, decompLen)
	}

	return decompLen, nil
}
