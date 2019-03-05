// Package lz4 implements goDB's Encoder interface for lz4 (de-)compression of flow data
package lz4

/*
#cgo CFLAGS: -O3
#define LZ4_STATIC_LINKING_ONLY

#include <stdlib.h>
#include <stdio.h>
#include "lz4hc.h"
#include "lz4.h"

int cCompress(int len, char *input, char *output, int level) {
  return LZ4_compressHC2(input, output, len, level);
}

int cUncompress(char *output, int out_len, char *input) {
  return LZ4_decompress_fast(input, output, out_len);
}
*/
import "C"

import (
	"errors"
	"io"
	"unsafe"
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

// Compress compresses the input data and writes it to dst
func (e *Encoder) Compress(data []byte, dst io.Writer) (n int, err error) {

	// LZ4 states that non-compressible data can be expanded to up to 0.4%.
	// This length bound is the conservative version of the bound specified in the LZ4 source
	var buf = make([]byte, int((1.004*float64(len(data)))+16))
	var compLen = int(C.cCompress(C.int(len(data)), (*C.char)(unsafe.Pointer(&data[0])), (*C.char)(unsafe.Pointer(&buf[0])), C.int(e.level)))

	// sanity check whether the computed worst case has been exceeded in C call
	if len(buf) < compLen {
		return n, errors.New("Buffer size mismatch for compressed data")
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
		return 0, errors.New("Incorrect number of bytes read from data source")
	}

	// decompress data
	return int(C.cUncompress((*C.char)(unsafe.Pointer(&out[0])), C.int(len(out)), (*C.char)(unsafe.Pointer(&in[0])))), nil
}
