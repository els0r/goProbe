// Package lz4 implements goDB's Encoder interface for lz4 (de-)compression of flow data
package lz4

import (
	"errors"

	"github.com/els0r/goProbe/v4/pkg/goDB/encoder/encoders"
)

const (
	MaxCompressionLevel     = 12 // MaxCompressionLevel denotes the maximum useful compression level
	defaultCompressionLevel = 6
)

var (

	// ErrBufferSizeMismatch denotes that the allocated buffer is insufficient in size
	ErrBufferSizeMismatch = errors.New("buffer size mismatch for compressed data")

	// ErrIncorrectNumBytesRead denotes that the number of bytes read during decompression
	// does not match the expected size
	ErrIncorrectNumBytesRead = errors.New("incorrect number of bytes read from data source during decompression")
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
		e.SetLevel(level)
	}
}

// SetLevel sets / changes the compression level (if supported)
func (e *Encoder) SetLevel(level int) {
	e.level = level
}

// Type will return the type of encoder
func (e *Encoder) Type() encoders.Type {
	return encoders.EncoderTypeLZ4
}
