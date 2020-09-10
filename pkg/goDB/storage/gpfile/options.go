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
