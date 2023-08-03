// Package zstd implements goDB's Encoder interface for ZStandard (de-)compression of flow data
package zstd

/*
#cgo linux CFLAGS: -O3
#cgo linux LDFLAGS: -O3 -lzstd
#cgo darwin,amd64 LDFLAGS: -O3 -lzstd
#cgo darwin,arm64 LDFLAGS: -O3 -lzstd
#include <stdint.h>
#include <zstd.h>

size_t zstdInitCCtx(uintptr_t cctx, const int level) {

	// set compression parameters
	size_t const levelStatus = ZSTD_CCtx_setParameter((ZSTD_CCtx*)cctx, ZSTD_c_compressionLevel, level);
	if (ZSTD_isError(levelStatus)) {
		return levelStatus;
	}
	size_t const checksumStatus = ZSTD_CCtx_setParameter((ZSTD_CCtx*)cctx, ZSTD_c_checksumFlag, 0);
	if (ZSTD_isError(checksumStatus)) {
		return checksumStatus;
	}

	return 0;
}

size_t zstdCompress(uintptr_t cctx, const char *src, const size_t srcSize, char *dst, const size_t dstSize) {
	return ZSTD_compress2((ZSTD_CCtx*)cctx, dst, dstSize, src, srcSize);
}

size_t zstdGetCompressBound(const size_t srcSize) {
	return ZSTD_compressBound(srcSize);
}

const char* zstdGetErrorName(const int code) {
	return ZSTD_getErrorName(code);
}

size_t zstdDecompress(uintptr_t dctx, const char *src, const size_t srcSize, char *dst, size_t dstSize) {
	return ZSTD_decompressDCtx((ZSTD_DCtx*)dctx, dst, dstSize, src, srcSize);
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

// Encoder compresses data with the ZStandard algorithm (omitting certain bounds-checks for performance reasons)
type Encoder struct {

	// compression context
	cCtx *C.ZSTD_CCtx

	// decompression context
	dCtx *C.ZSTD_DCtx

	// compression level
	level int
}

// Option sets additional parameters on the Encoder
type Option func(*Encoder)

// New creates a new ZStandard Encoder that can be used to compress/decompress data
func New(opts ...Option) *Encoder {
	// compression level of 6 is used by default as it offers higher compression speeds than maximum compression,
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
		e.SetLevel(level)
	}
}

// SetLevel sets / changes the compression level (if supported)
func (e *Encoder) SetLevel(level int) {
	e.level = level
}

// Type will return the type of encoder
func (e *Encoder) Type() encoders.Type {
	return encoders.EncoderTypeZSTD
}

// Close will close the encoder and release potentially allocated resources
func (e *Encoder) Close() error {
	if e.dCtx != nil {
		if fErr := int(C.ZSTD_freeDCtx(e.dCtx)); fErr < 0 {
			errName := C.GoString(C.zstdGetErrorName(C.int(fErr)))
			return fmt.Errorf("zstd: decompression context release failed: %s", errName)
		}
	}
	if e.cCtx != nil {
		if fErr := int(C.ZSTD_freeCCtx(e.cCtx)); fErr < 0 {
			errName := C.GoString(C.zstdGetErrorName(C.int(fErr)))
			return fmt.Errorf("zstd: compression context release failed: %s", errName)
		}
	}
	return nil
}

// Compress compresses the input data and writes it to dst
func (e *Encoder) Compress(data, buf []byte, dst io.Writer) (n int, err error) {
	dstCapacity := int(C.zstdGetCompressBound(
		C.size_t(len(data)),
	))

	// If no compression context exists, create one
	if e.cCtx == nil {
		if e.cCtx = C.ZSTD_createCCtx(); e.cCtx == nil {
			return n, fmt.Errorf("zstd: compression context creation failed")
		}
		if errCtx := int(C.zstdInitCCtx(C.uintptr_t(uintptr(unsafe.Pointer(e.cCtx))), C.int(e.level))); errCtx < 0 {
			errName := C.GoString(C.zstdGetErrorName(C.int(errCtx)))
			return n, fmt.Errorf("zstd: compression context init failed: %s", errName)
		}
	}

	if cap(buf) < dstCapacity {
		buf = make([]byte, 0, 2*dstCapacity)
	}
	buf = buf[:dstCapacity]

	compLen := int(C.zstdCompress(
		C.uintptr_t(uintptr(unsafe.Pointer(e.cCtx))),
		(*C.char)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		(*C.char)(unsafe.Pointer(&buf[0])),
		C.size_t(dstCapacity)))

	// properly handle error codes from zsdt
	if compLen < 0 {
		errName := C.GoString(C.zstdGetErrorName(C.int(compLen)))
		return n, fmt.Errorf("zstd: compression failed: %s (%d)", errName, compLen)
	}

	// sanity check whether the computed worst case has been exceeded in C call
	if len(buf) < compLen {
		return n, errors.New("zstd: buffer size mismatch for compressed data")
	}

	if n, err = dst.Write(buf[:compLen]); err != nil {
		return n, err
	}

	return n, nil
}

// Decompress runs ZStandard decompression on "in" read from "src" and writes it to "out"
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
		return 0, errors.New("zstd: incorrect number of bytes read from data source")
	}

	// If no decompression context exists, create one
	if e.dCtx == nil {
		if e.dCtx = C.ZSTD_createDCtx(); e.dCtx == nil {
			return 0, fmt.Errorf("zstd: decompression context creation failed")
		}
	}

	// decompress data
	decompLen := int(C.zstdDecompress(
		C.uintptr_t(uintptr(unsafe.Pointer(e.dCtx))),
		(*C.char)(unsafe.Pointer(&in[0])),
		C.size_t(len(in)),
		(*C.char)(unsafe.Pointer(&out[0])),
		C.size_t(len(out))))
	if decompLen < 0 {
		errName := C.GoString(C.zstdGetErrorName(C.int(decompLen)))
		return 0, fmt.Errorf("zstd: decompression failed: %s (%d)", errName, decompLen)
	}

	return decompLen, nil
}
