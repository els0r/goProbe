package gpfile

import "github.com/els0r/goProbe/pkg/goDB/encoder"

// Option defines optional arguments to gpfile
type Option func(*GPFile)

// WithEncoder allows to set the compression implementation
func WithEncoder(e encoder.Encoder) Option {
	return func(g *GPFile) {
		g.defaultEncoder = e
		g.defaultEncoderType = e.Type()
		g.freeEncoder = false
	}
}

// WithReadAll triggers a full read of the underlying file from disk
// upon first read access to minimize I/O load.
// Seeking is handled by replacing the underlying file with a seekable
// in-memory structure (c.f. readWriteSeekCloser interface)
func WithReadAll(pool MemPoolGCable) Option {
	return func(g *GPFile) {
		g.memPool = pool
	}
}
