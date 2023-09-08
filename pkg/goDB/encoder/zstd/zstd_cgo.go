//go:build cgo
// +build cgo

package zstd

/*
#cgo linux CFLAGS: -O3
#cgo linux LDFLAGS: -O3 -lzstd
#cgo darwin,amd64 LDFLAGS: -O3 -lzstd
#cgo darwin,arm64 LDFLAGS: -O3 -lzstd
#include <stdint.h>
#include <zstd.h>

size_t zstdInitCCtx(ZSTD_CCtx* cctx, const int level) {

	// set compression parameters
	size_t status;
	status = ZSTD_CCtx_setParameter(cctx, ZSTD_c_compressionLevel, level);
	if (ZSTD_isError(status)) {
		return status;
	}
	status = ZSTD_CCtx_setParameter(cctx, ZSTD_c_checksumFlag, 0);
	if (ZSTD_isError(status)) {
		return status;
	}
	status = ZSTD_CCtx_setParameter(cctx, ZSTD_c_contentSizeFlag, 0);
	if (ZSTD_isError(status)) {
		return status;
	}
	status = ZSTD_CCtx_setParameter(cctx, ZSTD_c_dictIDFlag, 0);
	if (ZSTD_isError(status)) {
		return status;
	}

	// This parameter would even remove the header / magic, but it is not yet stable and hence
	// cannot be expected to be available
	// status = ZSTD_CCtx_setParameter(cctx, ZSTD_c_format, ZSTD_f_zstd1_magicless);
	// if (ZSTD_isError(status)) {
	// 	return status;
	// }

	return 0;
}

// The following wrappers are required to avoid memory allocations when handing over the compression /
// decompression context from Go to CGO

size_t zstdCompress(uintptr_t cctx, const char *src, const size_t srcSize, char *dst, const size_t dstSize) {
    return ZSTD_compress2((ZSTD_CCtx*)cctx, dst, dstSize, src, srcSize);
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
)

// Encoder compresses data with the ZStandard algorithm (omitting certain bounds-checks for performance reasons)
type Encoder struct {

	// compression context
	cCtx *C.ZSTD_CCtx

	// decompression context
	dCtx *C.ZSTD_DCtx

	// compression level
	level int
}

// Close will close the encoder and release potentially allocated resources
func (e *Encoder) Close() error {
	if e.dCtx != nil {
		if fErr := int(C.ZSTD_freeDCtx(e.dCtx)); fErr < 0 {
			errName := C.GoString(C.ZSTD_getErrorName(C.ulong(fErr)))
			return fmt.Errorf("zstd: decompression context release failed: %s", errName)
		}
	}
	if e.cCtx != nil {
		if fErr := int(C.ZSTD_freeCCtx(e.cCtx)); fErr < 0 {
			errName := C.GoString(C.ZSTD_getErrorName(C.ulong(fErr)))
			return fmt.Errorf("zstd: compression context release failed: %s", errName)
		}
	}
	return nil
}

// Compress compresses the input data and writes it to dst
func (e *Encoder) Compress(data, buf []byte, dst io.Writer) (n int, err error) {

	// Handle output slice size
	dstCapacity := int(C.ZSTD_compressBound(
		C.size_t(len(data)),
	))
	if cap(buf) < dstCapacity {
		buf = make([]byte, 0, 2*dstCapacity)
	}
	buf = buf[:dstCapacity]

	// If no compression context exists, create one
	if e.cCtx == nil {
		if e.cCtx = C.ZSTD_createCCtx(); e.cCtx == nil {
			return n, fmt.Errorf("zstd: compression context creation failed")
		}
		if errCtx := int(C.zstdInitCCtx(e.cCtx, C.int(e.level))); errCtx < 0 {
			errName := C.GoString(C.ZSTD_getErrorName(C.ulong(errCtx)))
			return n, fmt.Errorf("zstd: compression context init failed: %s", errName)
		}
	}

	// Handle special case of empty input data
	var dataPtr unsafe.Pointer
	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}

	// Compress data
	compLen := int(C.zstdCompress(
		C.uintptr_t(uintptr(unsafe.Pointer(e.cCtx))),
		(*C.char)(dataPtr),
		C.ulong(len(data)),
		(*C.char)(unsafe.Pointer(&buf[0])),
		C.ulong(dstCapacity)))
	if compLen < 0 {
		errName := C.GoString(C.ZSTD_getErrorName(C.ulong(compLen)))
		return n, fmt.Errorf("zstd: compression failed: %s (%d)", errName, compLen)
	}

	// Perform sanity check whether the computed worst case has been exceeded in C call
	if len(buf) < compLen {
		return n, errors.New("zstd: buffer size mismatch for compressed data")
	}

	// If provided, write output to the writer
	if dst != nil {
		if n, err = dst.Write(buf[:compLen]); err != nil {
			return n, err
		}
	}

	return n, nil
}

// Decompress runs ZStandard decompression on "in" read from "src" and writes it to "out"
func (e *Encoder) Decompress(in, out []byte, src io.Reader) (int, error) {
	var nBytesConsumed int

	// Read compressed source data
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

	// Handle special case of empty output data
	var outPtr unsafe.Pointer
	if len(out) > 0 {
		outPtr = unsafe.Pointer(&out[0])
	}

	// Decompress data
	decompLen := int(C.zstdDecompress(
		C.uintptr_t(uintptr(unsafe.Pointer(e.dCtx))),
		(*C.char)(unsafe.Pointer(&in[0])),
		C.size_t(len(in)),
		(*C.char)(outPtr),
		C.size_t(len(out))))
	if decompLen < 0 {
		errName := C.GoString(C.ZSTD_getErrorName(C.ulong(decompLen)))
		return 0, fmt.Errorf("zstd: decompression failed: %s (%d)", errName, decompLen)
	}

	return decompLen, nil
}
