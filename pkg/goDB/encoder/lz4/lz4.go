// Package lz4 implements goDB's Encoder interface for lz4 (de-)compression of flow data
package lz4

/*
#cgo linux LDFLAGS: -llz4
#cgo darwin,amd64 LDFLAGS: -llz4
#cgo darwin,arm64 LDFLAGS: -llz4
#include <stdlib.h>
#include <stdio.h>
#include "lz4hc.h"
#include "lz4frame.h"
#include "lz4.h"

int cCompress(char *src, int srcSize, char *dst, int level) {
	// initialize the frame compression preferences
	// taken from https://github.com/lz4/lz4/blob/dev/examples/frameCompress.c
	const LZ4F_preferences_t prefs = {
    	{
			LZ4F_default,
			LZ4F_blockLinked,
			LZ4F_noContentChecksum,
			LZ4F_frame,
			0,
			0,
			LZ4F_noBlockChecksum,
		},
		level,
		0,
		0,
		{ 0, 0, 0 },
	};

	return LZ4F_compressFrame(dst, LZ4F_compressFrameBound(srcSize, &prefs), src, srcSize, &prefs);
}

static const LZ4F_decompressOptions_t decompOpts = {
	1, // pledges that last 64KB decompressed data will remain available unmodified between invocations
	1, // disable checksum calculation and verification to save CPU time. This line is why we even choose to use liblz4 1.9.4
	0, // reserved 0
	0, // reserved 1
};

int cDecompress(char *src, int srcSize, char *dst, int dstSize) {
	// create decompression context
	LZ4F_dctx* ctx;

	// check if context creation was successful
	size_t const dctxStatus = LZ4F_createDecompressionContext(&ctx, LZ4F_VERSION);
	if (LZ4F_isError(dctxStatus)) {
		return -1;
	}
	if (!ctx) {
		return -1;
	}


	// actual decompression
	size_t sSize = srcSize;
	size_t dSize = dstSize;

	size_t result = LZ4F_decompress(ctx, dst, &dSize, src, &sSize, &decompOpts);

	// release context
	LZ4F_freeDecompressionContext(ctx);

	if (LZ4F_isError(result)) {
		return -1;
	}

	// LZ4_decompress writes the amount of decompressed bytes into dSize
	return dSize;
}
*/
import "C"

import (
	"errors"
	"io"
	"unsafe"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
)

// Encoder compresses data with the LZ4 algorithm (omitting certain bounds-checks for performance reasons)
type Encoder struct {
	// compression level
	level int
}

// Option sets additional parameters on the Encoder
type Option func(*Encoder)

// New creates a new LZ4 Encoder that can be used to compress/decompress data
func New(opts ...Option) *Encoder {
	// compression level of 512 is used by default
	l := &Encoder{level: 512}

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

	// LZ4 states that non-compressible data can be expanded to up to 0.4%.
	// This length bound is the conservative version of the bound specified in the LZ4 source
	var buf = make([]byte, int((1.004*float64(len(data)))+16))

	var compLen = int(C.cCompress(
		(*C.char)(unsafe.Pointer(&data[0])),
		C.int(len(data)),
		(*C.char)(unsafe.Pointer(&buf[0])),
		C.int(e.level)),
	)

	// sanity check whether the computed worst case has been exceeded in C call
	if len(buf) < compLen {
		return n, errors.New("buffer size mismatch for compressed data")
	}

	if n, err = dst.Write(buf[0:compLen]); err != nil {
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
		return 0, errors.New("incorrect number of bytes read from data source")
	}

	// decompress data
	nBytesDecompressed := int(C.cDecompress(
		(*C.char)(unsafe.Pointer(&in[0])),
		C.int(len(in)),
		(*C.char)(unsafe.Pointer(&out[0])),
		C.int(len(out))),
	)
	if nBytesDecompressed < 0 {
		return 0, errors.New("invalid LZ4 data detected during decompression")
	}

	return nBytesDecompressed, nil
}
