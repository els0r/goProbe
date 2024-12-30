package encoder

import (
	"errors"
	"fmt"
	"io"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/encoder/lz4"
	"github.com/els0r/goProbe/pkg/goDB/encoder/null"
	"github.com/els0r/goProbe/pkg/goDB/encoder/zstd"
)

var (

	// ErrDeprecatedLZ4Cust is used to signify the obsoletion of the lz4cust encoding
	ErrDeprecatedLZ4Cust = errors.New("the LZ4 custom implementation has been deprecated, please check out the v4.0.0 README")
)

// Encoder provides the GP File with a means to compress and decompress its raw data
type Encoder interface {

	// Type will return the type of encoder
	Type() encoders.Type

	// Close will close the encoder and release potentially allocated resources
	Close() error

	// Compress will take the input data slice and write it to dst. The number of written compressed bytes is returned with n
	Compress(data, buf []byte, dst io.Writer) (n int, err error)

	// Decompress reads compressed bytes from src into in, decompresses it into out and returns the number of bytes decompressed.
	// It is the responsibility of the caller to ensure that in and out are properly sized
	Decompress(in, out []byte, src io.Reader) (n int, err error)

	// SetLevel sets / changes the compression level (if supported)
	SetLevel(level int)
}

// New creates a new encoder based on an encoder type
func New(t encoders.Type) (Encoder, error) {
	switch t {
	case encoders.EncoderTypeNull:
		return null.New(), nil
	case encoders.EncoderTypeLZ4:
		return lz4.New(), nil
	case encoders.EncoderTypeLZ4CustomDeprecated:
		return nil, ErrDeprecatedLZ4Cust
	case encoders.EncoderTypeZSTD:
		return zstd.New(), nil
	default:
		return nil, fmt.Errorf("unsupported encoder: %v", t)
	}
}

// NewByString is a convenience method for encoder selection by string
// rather than enumeration code
func NewByString(t string) (Encoder, error) {
	et, err := encoders.GetTypeByString(t)
	if err != nil {
		return nil, err
	}
	return New(et)
}
