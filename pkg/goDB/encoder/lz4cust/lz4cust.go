// Package lz4cust implements goDB's Encoder interface for lz4 (de-)compression of flow data
package lz4cust

import (
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
)

// Encoder compresses data with the LZ4 algorithm (omitting certain bounds-checks for performance reasons)
type Encoder struct {
	// compression level
	level int
}

// Option sets additional parameters on the Encoder
type Option func(*Encoder)

// SetLevel sets / changes the compression level (if supported)
func (e *Encoder) SetLevel(level int) {
	e.level = level
}

// Type will return the type of encoder
func (e *Encoder) Type() encoders.Type {
	return encoders.EncoderTypeLZ4CustomDeprecated
}

// Close will close the encoder and release potentially allocated resources
func (e *Encoder) Close() error {
	return nil
}
