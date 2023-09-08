package null

import (
	"errors"
	"io"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
)

// DefaultEncoder proivdes a globally usable null encoder / DefaultEncoder
// Since all null compression / decompression actions are stateless it is safe
var DefaultEncoder = New()

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

// Close will close the encoder and release potentially allocated resources
func (e *Encoder) Close() error {
	return nil
}

// Compress directly writes "data" to "dst" without any further manipulation
func (e *Encoder) Compress(data, _ []byte, dst io.Writer) (n int, err error) {
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
		return 0, errors.New("incorrect number of bytes read from data source")
	}

	return n, nil
}

// SetLevel sets / changes the compression level (if supported)
func (e *Encoder) SetLevel(_ int) {}
