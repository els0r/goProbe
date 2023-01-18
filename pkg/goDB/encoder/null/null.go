package null

import (
	"errors"
	"io"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
)

// Encoder compresses data without any algorithm
type Encoder struct{}

// New creates a new Null encoder which does not manipulate the original data
// in any way. It's meant to be used where no compression is desired
func New() *Encoder {
	return &Encoder{}
}

// Type will return the type of encoder
func (e *Encoder) Type() encoders.Type {
	return encoders.EncoderTypeNull
}

// Compress directly writes "data" to "dst" without any further manipulation
func (e *Encoder) Compress(data []byte, dst io.Writer) (n int, err error) {
	return dst.Write(data)
}

// Decompress runs no decompression on the data read from src. It's directly written
// to "out"
func (e *Encoder) Decompress(_, out []byte, src io.Reader) (n int, err error) {
	n, err = src.Read(out)
	if err != nil {
		return 0, err
	}
	if n != len(out) {
		return 0, errors.New("Incorrect number of bytes read from data source")
	}

	return n, nil
}
