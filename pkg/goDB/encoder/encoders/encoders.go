package encoders

import (
	"fmt"
	"strings"
)

// Type denotes the type of encoder
type Type int

// Enumeration of directions to be considered
const (
	EncoderTypeLZ4  Type = iota // LZ4 encoder / compressor (default, hence allocated the value 0)
	EncoderTypeNull             // Null encoder
	EncoderTypeZSTD             // ZSTD encoder / compressor

	// should always be the last entry
	MaxEncoderType = EncoderTypeZSTD
)

var encoderNames = map[Type]string{
	EncoderTypeLZ4:  "lz4",
	EncoderTypeNull: "null",
	EncoderTypeZSTD: "zstd",
}

// String returns a string representation of the encoding type
func (t Type) String() string {
	return encoderNames[t]
}

// GetTypeByString returns the encoder type based on a named string
func GetTypeByString(t string) (Type, error) {
	switch strings.ToLower(t) {
	case "null", "":
		return EncoderTypeNull, nil
	case "lz4":
		return EncoderTypeLZ4, nil
	case "zstd":
		return EncoderTypeZSTD, nil
	default:
		return EncoderTypeNull, fmt.Errorf("Unsupported encoder: %v", t)
	}
}
