package null

import (
	"errors"
	"io"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
)

// Encoder compresses data without any algorithm
type Encoder struct{}

// New creates a new LZ4 Encoder that can be used to compress/decompress data
func New() *Encoder {
	return &Encoder{}
}

// Type will return the type of encoder
func (e *Encoder) Type() encoders.Type {
	return encoders.EncoderTypeNull
}

// Compress compresses the input data and writes it to dst
func (e *Encoder) Compress(data []byte, dst io.Writer) (n int, err error) {
	if n, err = dst.Write(data); err != nil {
		return n, err
	}

	return n, nil
}

// Decompress runs LZ4 decompression on "in" read from "src" and writes it to "out"
func (e *Encoder) Decompress(in, out []byte, src io.Reader) (n int, err error) {
	n, err = src.Read(out)
	if err != nil {
		return 0, err
	}
	if n != len(out) {
		return 0, errors.New("Incorrect number of bytes read from data source")
	}

	return n, nil
}
