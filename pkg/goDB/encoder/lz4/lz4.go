// Package lz4 implements goDB's Encoder interface for lz4 (de-)compression of flow data
package lz4

/*
#cgo linux CFLAGS: -O2 -g
#cgo linux LDFLAGS: -llz4
#cgo darwin,amd64 LDFLAGS: -llz4
#cgo darwin,arm64 LDFLAGS: -llz4
#include <stdlib.h>
#include <stdio.h>
#include "lz4frame.h"

size_t cCompress(const void *src, size_t srcSize, void *dst, size_t dstSize, int level) {
	// initialize the frame compression preferences
	// taken from https://github.com/lz4/lz4/blob/dev/examples/frameCompress.c
	const LZ4F_preferences_t prefs = {
		{
			LZ4F_max256KB,
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

size_t getCompressBound(size_t srcSize, int level) {
	const LZ4F_preferences_t prefs = {
		{
			LZ4F_max256KB,
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

const char* getErrorName(int code) {
	return LZ4F_getErrorName(code);
}

size_t cDecompress(const void *src, size_t srcSize, void *dst, size_t dstSize) {
	// create decompression context
	LZ4F_dctx* ctx;

	// check if context creation was successful
	size_t const dctxStatus = LZ4F_createDecompressionContext(&ctx, LZ4F_VERSION);
	if (LZ4F_isError(dctxStatus)) {
		return dctxStatus;
	}
	if (!ctx) {
		return -1;
	}

	// actual decompression
	// read from src until there are no more bytes to be read
	const void* srcPtr = (const char*)src;
	const void* const srcEnd = (const char*)src + srcSize;
	size_t result = 1;
	while (srcPtr < srcEnd && result != 0) {
		size_t sSize = (const char*)srcEnd - (const char*)srcPtr;

		result = LZ4F_decompress(ctx, dst, &dstSize, src, &sSize, &decompOpts);
		if (LZ4F_isError(result)) {
			return result;
		}
		srcPtr = (const char*)srcPtr + sSize;
	}

	// release context
	LZ4F_freeDecompressionContext(ctx);

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

const defaultCompressionLevel = 4

// Encoder compresses data with the LZ4 algorithm (omitting certain bounds-checks for performance reasons)
type Encoder struct {
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

// Compress compresses the input data and writes it to dst
func (e *Encoder) Compress(data []byte, dst io.Writer) (n int, err error) {
	dstCapacity := int(C.getCompressBound(
		C.size_t(len(data)),
		C.int(e.level),
	))

	var dstBuf = make([]byte, dstCapacity)

	var compLen = int(C.cCompress(
		unsafe.Pointer(&data[0]),
		C.size_t(len(data)),
		unsafe.Pointer(&dstBuf[0]),
		C.size_t(dstCapacity),
		C.int(e.level)),
	)

	// properly handle error codes from lz4
	if compLen < 0 {
		errName := C.GoString(C.getErrorName(C.int(compLen)))
		return n, fmt.Errorf("lz4: compression failed: %s (%d)", errName, compLen)
	}

	// sanity check whether the computed worst case has been exceeded in C call
	if len(dstBuf) < compLen {
		return n, errors.New("lz4: buffer size mismatch for compressed data")
	}

	// TODO: Debug. Remove
	fmt.Printf("comp: %x\n", dstBuf[:4])

	if n, err = dst.Write(dstBuf[0:compLen]); err != nil {
		return n, err
	}

	return n, nil
}

// Decompress runs LZ4 decompression on "in" read from "src" and writes it to "out"
func (e *Encoder) Decompress(in, out []byte, src io.Reader) (n int, err error) {
	var nBytesRead int

	// read compressed source data
	nBytesRead, err = src.Read(in)
	if err != nil {
		return 0, err
	}
	if nBytesRead != len(in) {
		return 0, errors.New("lz4: incorrect number of bytes read from data source")
	}

	// TODO: Debug. Remove
	fmt.Printf("dec: %x\n", in[:4])

	// decompress data
	nBytesDecompressed := int(C.cDecompress(
		unsafe.Pointer(&in[0]),
		C.size_t(len(in)),
		unsafe.Pointer(&out[0]),
		C.size_t(len(out))),
	)
	if nBytesDecompressed < 0 {
		errName := C.GoString(C.getErrorName(C.int(nBytesDecompressed)))
		return 0, fmt.Errorf("lz4: decompression failed: %s (%d)", errName, nBytesDecompressed)
	}

	return nBytesDecompressed, nil
}
