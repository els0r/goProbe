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

	// should always be the last entry
	MaxEncoderType = EncoderTypeNull
)

func GetTypeByString(t string) (Type, error) {
	switch strings.ToLower(t) {
	case "null", "":
		return EncoderTypeNull, nil
	case "lz4":
		return EncoderTypeLZ4, nil
	default:
		return EncoderTypeNull, fmt.Errorf("Unsupported encoder: %v", t)
	}
}
