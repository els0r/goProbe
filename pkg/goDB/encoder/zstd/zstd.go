// Package zstd implements goDB's Encoder interface for ZStandard (de-)compression of flow data
package zstd

import (
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
)

const (
	MaxCompressionLevel     = 19 // MaxCompressionLevel denotes the maximum useful compression level
	defaultCompressionLevel = 6
)

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
