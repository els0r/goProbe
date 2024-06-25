package client

import (
	"bufio"
	"io"
)

type eventStream struct {
	scanner     *bufio.Scanner
	wordScanner *bufio.Scanner
	bufSize     int
}

func newEventStream(r io.Reader) *eventStream {
	s := bufio.NewScanner(r)
	wordScanner := bufio.NewScanner(r)
	return &eventStream{scanner: s, wordScanner: wordScanner}
}

func (e *eventStream) setBuffer(size int) *eventStream {
	e.scanner.Buffer(make([]byte, size), bufio.MaxScanTokenSize)
	return e
}
