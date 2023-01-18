package gpfile

import "github.com/els0r/goProbe/pkg/goDB/encoder/encoders"

// Option defines optional arguments to gpfile
type Option func(*GPFile)

// WithEncoder allows to set the compression implementation
func WithEncoder(e encoders.Type) Option {
	return func(g *GPFile) {
		g.defaultEncoderType = e
		return
	}
}

// WithHighCardinalityEncoder allows to set the compression implementation for
// high cardinality columns. What constitutes high cardinality is determined
// internally
func WithHighCardinalityEncoder(e encoders.Type) Option {
	return func(g *GPFile) {
		g.defaultHighCardintalityEncoderType = e
		return
	}
}
