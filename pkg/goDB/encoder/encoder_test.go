package encoder

import (
	"testing"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
)

func TestNewByString(t *testing.T) {

	var tests = []struct {
		name              string
		encoderTypeString string
		expect            encoders.Type
		shouldFail        bool
	}{
		{"empty string", "", encoders.EncoderTypeNull, false},
		{"null encoder", "null", encoders.EncoderTypeNull, false},
		{"lz4 encoder", "lz4", encoders.EncoderTypeLZ4, false},
		{"lz4 encoder (uppercase)", "LZ4", encoders.EncoderTypeLZ4, false},
		{"unsupported encoder", "iwillneverbesupported", encoders.EncoderTypeNull, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e, err := NewByString(test.encoderTypeString)
			if test.shouldFail {
				if err == nil {
					t.Fatalf("expected to fail but didn't")
				}
			} else {
				if err != nil {
					t.Fatalf("failed to create encoder: %v", err)
				}

				if e.Type() != test.expect {
					t.Fatalf("have: %v; expect: %v", e.Type(), test.expect)
				}
			}
		})
	}
}
