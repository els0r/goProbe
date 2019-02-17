package goDB

import "io"

// Encoder provides the GP File with a means to compress and decompress its raw data
type Encoder interface {
	// Compress will take the input data slice and write it to dst. The number of written compressed bytes is returned with n
	Compress(data []byte, dst io.Writer) (n int, err error)
	// Decompress reads compressed bytes from src into in, decompresses it into out and returns the number of bytes decompressed. It is the responsibility of the caller to ensure that in and out are properly sized
	Decompress(in, out []byte, src io.Reader) (n int, err error)
}
