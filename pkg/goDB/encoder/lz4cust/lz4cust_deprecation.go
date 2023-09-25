//go:build !cgo || goprobe_nolz4cust
// +build !cgo goprobe_nolz4cust

package lz4cust

import (
	"errors"
	"io"
)

var (

	// ErrDeprecated is used to signify the obsoletion of the lz4cust encoding
	ErrDeprecated = errors.New("the LZ4 custom implementation has been deprecated, please check out the v4.0.0 README")
)

// New creates a dummy LZ4 Encoder that does nothing
func New(opts ...Option) *Encoder {
	return nil
}

// Compress is a stub to honor the encoder interface
func (e *Encoder) Compress(data, buf []byte, dst io.Writer) (n int, err error) {
	return -1, nil
}

// Decompress is a stub to honor the encoder interface
func (e *Encoder) Decompress(in, out []byte, src io.Reader) (n int, err error) {
	return -1, nil
}
