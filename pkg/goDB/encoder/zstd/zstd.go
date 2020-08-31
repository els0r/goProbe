package zstd

import (
	"errors"
	"io"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/valyala/gozstd"
)

// Encoder compresses data with the ZSTD algorithm
type Encoder struct {
	// compression level
	level int
}

// Option sets additional parameters on the Encoder
type Option func(*Encoder)

// New creates a new LZ4 Encoder that can be used to compress/decompress data
func New(opts ...Option) *Encoder {
	// compression level of 512 is used by default
	l := &Encoder{level: 5}

	// apply options
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// WithCompressionLevel allows the level to be set to something other than the default 512
func WithCompressionLevel(level int) Option {
	return func(e *Encoder) {
		e.level = level
	}
}

// Type will return the type of encoder
func (e *Encoder) Type() encoders.Type {
	return encoders.EncoderTypeZSTD
}

// Compress compresses the input data and writes it to dst
func (e *Encoder) Compress(data []byte, dst io.Writer) (n int, err error) {

	compData := gozstd.CompressLevel(nil, data, e.level)

	if n, err = dst.Write(compData); err != nil {
		return n, err
	}

	return n, nil
}

// Decompress runs LZ4 decompression on "in" read from "src" and writes it to "out"
func (e *Encoder) Decompress(in, out []byte, src io.Reader) (n int, err error) {
	var nBytesRead int

	// read compressed source data
	nBytesRead, err = src.Read(in)
	if err != nil {
		return 0, err
	}
	if nBytesRead != len(in) {
		return 0, errors.New("Incorrect number of bytes read from data source")
	}

	_, err = gozstd.Decompress(out[:0], in)

	return len(out), err
}
