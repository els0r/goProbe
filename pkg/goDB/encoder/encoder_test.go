package encoder

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/encoder/lz4"
	"github.com/els0r/goProbe/pkg/goDB/encoder/zstd"
)

var testEncoders = []encoders.Type{
	encoders.EncoderTypeNull,
	encoders.EncoderTypeLZ4,
	encoders.EncoderTypeLZ4Custom,
	encoders.EncoderTypeZSTD,
}

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
		{"lz4 encoder custom", "lz4cust", encoders.EncoderTypeLZ4Custom, false},
		{"lz4 encoder (uppercase)", "LZ4", encoders.EncoderTypeLZ4, false},
		{"zstd encoder", "zstd", encoders.EncoderTypeZSTD, false},
		{"zstd encoder (uppercase)", "ZSTD", encoders.EncoderTypeZSTD, false},
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

func TestCompressionDecompression(t *testing.T) {
	var nBytes = int64(len(encodingCorpus))

	for _, encType := range testEncoders {
		t.Run(encType.String(), func(t *testing.T) {
			enc, err := New(encType)
			if err != nil {
				t.Fatalf("Failed to instantiate encoder of type %s: %s", encType, err)
			}
			defer func() {
				if err := enc.Close(); err != nil {
					t.Fatalf("Failed to release encoder of type %s: %s", encType, err)
				}
			}()
			buf := bytes.NewBuffer(nil)
			nCompressed, err := enc.Compress(encodingCorpus, nil, buf)
			if err != nil {
				t.Fatalf("Failed to compress data for encoder of type %s: %s", encType, err)
			}
			if nCompressed != buf.Len() {
				t.Fatalf("Unexpected number of compressed bytes, want %d, have %d", buf.Len(), nCompressed)
			}

			out := make([]byte, nBytes)
			in := make([]byte, nCompressed)
			nDecompressed, err := enc.Decompress(in, out, buf)
			if err != nil {
				t.Fatalf("Failed to decompress data for encoder of type %s: %s", encType, err)
			}
			if nDecompressed != int(nBytes) {
				t.Fatalf("Unexpected number of decompressed bytes, want %d, have %d", nBytes, nDecompressed)
			}

			if string(out) != string(encodingCorpus) {
				t.Fatalf("Invalid data detected after round-trip")
			}
		})
	}
}

func TestCompressionDecompressionCustomLevel(t *testing.T) {
	var nBytes = int64(len(encodingCorpus))

	for _, encType := range testEncoders {
		for level := 0; level <= 12; level++ {
			t.Run(fmt.Sprintf("%s_%d", encType, level), func(t *testing.T) {
				enc, err := New(encType)
				if err != nil {
					t.Fatalf("Failed to instantiate encoder of type %s: %s", encType, err)
				}
				enc.SetLevel(level)
				defer func() {
					if err := enc.Close(); err != nil {
						t.Fatalf("Failed to release encoder of type %s: %s", encType, err)
					}
				}()
				buf := bytes.NewBuffer(nil)
				nCompressed, err := enc.Compress(encodingCorpus, nil, buf)
				if err != nil {
					t.Fatalf("Failed to compress data for encoder of type %s: %s", encType, err)
				}
				if nCompressed != buf.Len() {
					t.Fatalf("Unexpected number of compressed bytes, want %d, have %d", buf.Len(), nCompressed)
				}

				out := make([]byte, nBytes)
				in := make([]byte, nCompressed)
				nDecompressed, err := enc.Decompress(in, out, buf)
				if err != nil {
					t.Fatalf("Failed to decompress data for encoder of type %s: %s", encType, err)
				}
				if nDecompressed != int(nBytes) {
					t.Fatalf("Unexpected number of decompressed bytes, want %d, have %d", nBytes, nDecompressed)
				}

				if string(out) != string(encodingCorpus) {
					t.Fatalf("Invalid data detected after round-trip")
				}
			})
		}
	}
}

func BenchmarkEncodersCompress(b *testing.B) {
	var nBytes = int64(len(encodingCorpus))

	for _, encType := range testEncoders {
		b.Run(encType.String(), func(b *testing.B) {
			enc, err := New(encType)
			if err != nil {
				b.Fatalf("Failed to instantiate encoder of type %s: %s", encType, err)
			}

			defer func(b *testing.B) {
				if err := enc.Close(); err != nil {
					b.Fatalf("Failed to close encoder of type %s: %s", encType, err)
				}
			}(b)

			b.ReportAllocs()
			b.SetBytes(nBytes)
			b.ResetTimer()
			buf := bytes.NewBuffer(nil)
			out := make([]byte, nBytes)

			for i := 0; i < b.N; i++ {
				_, _ = enc.Compress(encodingCorpus, out, buf)
				_ = buf
				buf.Reset()
			}
		})
	}
}

func BenchmarkEncodersDecompress(b *testing.B) {
	var nBytes = int64(len(encodingCorpus))

	for _, encType := range testEncoders {
		b.Run(encType.String(), func(b *testing.B) {
			enc, err := New(encType)
			if err != nil {
				b.Fatalf("Failed to instantiate encoder of type %s: %s", encType, err)
			}
			defer func() {
				if err := enc.Close(); err != nil {
					b.Fatalf("Failed to release encoder of type %s: %s", encType, err)
				}
			}()
			buf := bytes.NewBuffer(nil)
			nWritten, err := enc.Compress(encodingCorpus, nil, buf)
			if err != nil {
				b.Fatalf("Failed to encode test data for encoder type %s: %s", encType, err)
			}

			b.ReportAllocs()
			b.SetBytes(int64(nWritten))
			b.ResetTimer()

			out := make([]byte, nBytes)
			in := make([]byte, nWritten)
			reader := bytes.NewReader(buf.Bytes())
			for i := 0; i < b.N; i++ {
				_, _ = enc.Decompress(in, out, reader)
				_ = in
				_ = out

				_, _ = reader.Seek(0, 0)
			}
		})
	}
}

func BenchmarkLevelsCompress(b *testing.B) {
	var nBytes = int64(len(encodingCorpus))

	encoders := map[encoders.Type]func(int) Encoder{
		encoders.EncoderTypeLZ4: func(level int) Encoder {
			return lz4.New(lz4.WithCompressionLevel(level))
		},
		encoders.EncoderTypeZSTD: func(level int) Encoder {
			return zstd.New(zstd.WithCompressionLevel(level))
		},
	}

	for encType, encFn := range encoders {
		for level := 1; level <= 12; level++ {
			b.Run(encType.String()+fmt.Sprintf("-lvl-%d", level), func(b *testing.B) {

				enc := encFn(level)
				defer func() {
					if err := enc.Close(); err != nil {
						b.Fatalf("Failed to release encoder of type %s: %s", encType, err)
					}
				}()

				b.ReportAllocs()
				b.SetBytes(nBytes)
				b.ResetTimer()
				buf := bytes.NewBuffer(nil)

				tmp := make([]byte, 1725)

				for i := 0; i < b.N; i++ {
					_, _ = enc.Compress(encodingCorpus, tmp, buf)
					_ = buf
					buf.Reset()
				}
			})
		}
	}
}

func BenchmarkLevelsDecompress(b *testing.B) {
	var nBytes = int64(len(encodingCorpus))

	encoders := map[encoders.Type]func(int) Encoder{
		encoders.EncoderTypeLZ4: func(level int) Encoder {
			return lz4.New(lz4.WithCompressionLevel(level))
		},
		encoders.EncoderTypeZSTD: func(level int) Encoder {
			return zstd.New(zstd.WithCompressionLevel(level))
		},
	}

	for encType, encFn := range encoders {
		for level := 1; level <= 12; level++ {
			b.Run(encType.String()+fmt.Sprintf("-lvl-%d", level), func(b *testing.B) {

				enc := encFn(level)
				defer func() {
					if err := enc.Close(); err != nil {
						b.Fatalf("Failed to release encoder of type %s: %s", encType, err)
					}
				}()

				b.ReportAllocs()
				b.SetBytes(nBytes)
				b.ResetTimer()
				buf := bytes.NewBuffer(nil)
				nWritten, err := enc.Compress(encodingCorpus, nil, buf)
				if err != nil {
					b.Fatalf("Failed to encode test data for encoder type %s: %s", encType, err)
				}

				out := make([]byte, nBytes)
				in := make([]byte, nWritten)
				reader := bytes.NewReader(buf.Bytes())
				for i := 0; i < b.N; i++ {
					_, _ = enc.Decompress(in, out, reader)
					_ = in
					_ = out

					_, _ = reader.Seek(0, 0)
				}
			})
		}
	}
}

////////////////////////////////////////////////////////////////////////////////

var encodingCorpus = []byte(`The Internet has developed into the primarymeans of communication, while ensuring availability and sta-bility is becoming an increasingly challenging task. Trafﬁcmonitoring enables network operators to comprehend thecomposition of trafﬁc ﬂowing through individual corporateand private networks, making it essential for planning, re-porting and debugging purposes. Classical packet capture andaggregation concepts (e.g. NetFlow) typically rely on centralizedcollection of trafﬁc metadata. With the proliferation of networkenabled devices and the resulting increase in data volume,such approaches suffer from scalability issues, often prohibitingthe transfer of raw metadata as such. This paper describesa decentralized approach, eliminating the need for a centralcollector and storing local views of network trafﬁc patternson the respective devices performing the capture. In order toallow for the analysis of captured data, queries formulatedby analysts are distributed across all devices. Processingtakes place in a parallelized fashion on the respective localdata. Consequently, instead of continually transferring rawmetadata, signiﬁcantly smaller aggregate results are sent toa central location which are then combined into the requestedﬁnal result. The proposed system describes a lightweight andscalable monitoring solution, enabling the efﬁcient use ofavailable system resources on the distributed devices, henceallowing for high performance, real-time trafﬁc analysis ona global scale. The solution was implemented and deployedglobally on hosts managed and maintained by a large managednetwork security services provider.`)
