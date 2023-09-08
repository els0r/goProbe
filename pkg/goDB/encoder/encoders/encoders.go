package encoders

import (
	"fmt"
	"strings"
)

// Type denotes the type of encoder
type Type uint8

// IMPORTANT:
// When implementing new encoders, make sure to add the type above MaxEncoderType. Otherwise, compatibility
// with existing databases is broken
const (
	EncoderTypeLZ4Custom Type = iota // EncoderTypeLZ4Custom : LZ4 encoder / compressor with custom checksum stripping (default, hence allocated the value 0)
	EncoderTypeNull                  // EncoderTypeNull : Null encoder
	EncoderTypeZSTD                  // EncoderTypeZSTD : ZSTD encoder / compressor
	EncoderTypeLZ4                   // EncoderTypeLZ4 ; LZ4 encoder / compressor based on available lz4 system library (1.9.4 recommended for performance)

	// MaxEncoderType should always be the last entry
	MaxEncoderType = EncoderTypeLZ4
)

var encoderNames = map[Type]string{
	EncoderTypeLZ4:       "lz4",
	EncoderTypeLZ4Custom: "lz4cust",
	EncoderTypeNull:      "null",
	EncoderTypeZSTD:      "zstd",
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
	case "lz4cust":
		return EncoderTypeLZ4Custom, nil
	case "zstd":
		return EncoderTypeZSTD, nil
	default:
		return EncoderTypeNull, fmt.Errorf("unsupported encoder: %v", t)
	}
}
