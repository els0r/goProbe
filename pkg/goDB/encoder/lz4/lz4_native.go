//go:build !cgo
// +build !cgo

package lz4

import (
	"errors"
	"fmt"
	"io"

	"github.com/pierrec/lz4/v4"
)

// Close will close the encoder and release potentially allocated resources
func (e *Encoder) Close() error {
	return nil
}

// Compress compresses the input data and writes it to dst
func (e *Encoder) Compress(data, buf []byte, dst io.Writer) (n int, err error) {

	// Handle output slice size
	dstCapacity := lz4.CompressBlockBound(len(data))
	if cap(buf) < dstCapacity {
		buf = make([]byte, 0, 2*dstCapacity)
	}
	buf = buf[:dstCapacity]

	// Compress data
	compLen, err := lz4.CompressBlockHC(data, buf, lz4.CompressionLevel(e.level), nil, nil)
	if err != nil {
		return n, fmt.Errorf("lz4: compression failed: %s (%d)", err, compLen)
	}

	// Perform sanity check whether the computed worst case has been exceeded
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

	// Decompress data
	decompLen, err := lz4.UncompressBlock(in, out)
	if err != nil {
		return 0, fmt.Errorf("lz4: decompression failed: %s (%d)", err, decompLen)
	}

	return decompLen, nil
}
