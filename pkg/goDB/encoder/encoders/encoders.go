package encoders

// Type denotes the type of encoder
type Type int

// Enumeration of directions to be considered
const (
	EncoderTypeLZ4  Type = iota // LZ4 encoder / compressor (default, hence allocated the value 0)
	EncoderTypeNull             // Null encoder

	MaxEncoderType = EncoderTypeNull
)
