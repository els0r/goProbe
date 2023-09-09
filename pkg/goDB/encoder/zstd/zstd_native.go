//go:build !cgo
// +build !cgo

package zstd

import (
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// Encoder compresses data with the ZStandard algorithm (omitting certain bounds-checks for performance reasons)
type Encoder struct {

	// compression context
	encoder *zstd.Encoder

	// decompression context
	decoder *zstd.Decoder

	// compression level
	level int
}

// Close will close the encoder and release potentially allocated resources
func (e *Encoder) Close() error {
	if e.decoder != nil {
		e.decoder.Close()
	}
	if e.encoder != nil {
		if err := e.encoder.Close(); err != nil {
			return fmt.Errorf("zstd: compressor release failed: %w", err)
		}
	}
	return nil
}

// Compress compresses the input data and writes it to dst
func (e *Encoder) Compress(data, buf []byte, dst io.Writer) (n int, err error) {

	// If no compression context exists, create one
	if e.encoder == nil {
		if e.encoder, err = zstd.NewWriter(nil,
			zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(e.level)),
			zstd.WithEncoderCRC(false),
			zstd.WithEncoderConcurrency(1),
		); err != nil {
			return n, fmt.Errorf("zstd: compression context init failed: %w", err)
		}
	}

	// Compress data
	encData := e.encoder.EncodeAll(data, buf)

	// If provided, write output to the writer
	if dst != nil {
		if n, err = dst.Write(encData); err != nil {
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
	if e.decoder == nil {
		if e.decoder, err = zstd.NewReader(nil,
			zstd.IgnoreChecksum(true),
			zstd.WithDecoderConcurrency(1),
		); err != nil {
			return 0, fmt.Errorf("zstd: decompression context init failed: %w", err)
		}
	}

	// decompress data
	out = out[:0]
	decData, err := e.decoder.DecodeAll(in, out)
	decompLen := len(decData)
	if err != nil {
		return 0, fmt.Errorf("zstd: decompression failed: %w (%d)", err, decompLen)
	}

	return decompLen, nil
}
