package encoder

import (
	"fmt"
	"io"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/encoder/lz4"
	"github.com/els0r/goProbe/pkg/goDB/encoder/null"
)

// Encoder provides the GP File with a means to compress and decompress its raw data
type Encoder interface {

	// Type will return the type of encoder
	Type() encoders.Type

	// Compress will take the input data slice and write it to dst. The number of written compressed bytes is returned with n
	Compress(data []byte, dst io.Writer) (n int, err error)

	// Decompress reads compressed bytes from src into in, decompresses it into out and returns the number of bytes decompressed. It is the responsibility of the caller to ensure that in and out are properly sized
	Decompress(in, out []byte, src io.Reader) (n int, err error)
}

// New creates a new encoder based on an encoder type
func New(t encoders.Type) (Encoder, error) {
	switch t {
	case encoders.EncoderTypeNull:
		return null.New(), nil
	case encoders.EncoderTypeLZ4:
		return lz4.New(), nil
	default:
		return nil, fmt.Errorf("Unsupported encoder: %v", t)
	}
}
