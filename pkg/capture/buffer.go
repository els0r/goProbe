package capture

import (
	"unsafe"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/fako1024/slimcap/capture"
	"golang.org/x/sys/unix"
)

const (

	// bufElementAddSize denotes the required fixed size component for the pktType and pktSize
	// values per element: 1 (pktType) + 4 (uint32)
	bufElementAddSize = 5
)

var (

	// Initial size of a buffer
	initialBufferSize = unix.Getpagesize()

	// Global memory pool used to minimize allocations
	memPool = gpfile.NewMemPoolNoLimit()
)

// LocalBuffer denotes a local packet buffer used to temporarily capture packets
// from the source (e.g. during rotation) to avoid a ring / kernel buffer overflow
type LocalBuffer struct {
	data []byte // continuous buffer slice

	sizeLimit   int // maximum size to which the buffer may grow
	snapLen     int // capture length / snaplen for the underlying packet source
	elementSize int // size of an individual element stored in the buffer

	bufPos int // current position in continuous buffer slice
}

// WithSizeLimit allows setting a custom maxium size to which the buffer may grow
// no further elements can be added via Add() if this limit is reached
func WithSizeLimit(limit int) func(l *LocalBuffer) {
	return func(l *LocalBuffer) {
		if limit > 0 {
			l.sizeLimit = limit
		}
	}
}

// NewLocalBuffer instantiates a new buffer
func NewLocalBuffer(captureHandle capture.SourceZeroCopy, opts ...func(l *LocalBuffer)) *LocalBuffer {
	p := captureHandle.NewPacket()
	obj := &LocalBuffer{
		snapLen:     len(p.IPLayer()),
		elementSize: len(p.IPLayer()) + bufElementAddSize, // snaplen + sizes for pktType and pktSize
		sizeLimit:   config.DefaultLocalBufferSizeLimit,
	}

	// Apply functional options, if any
	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

// Add adds an element to the buffer, returning ok = true if successful
// If the buffer is full / may not grow any further, ok is false
func (l *LocalBuffer) Add(ipLayer capture.IPLayer, pktType byte, pktSize uint32) (ok bool) {

	// Lazily allocate memory only if a packets is added and fetch a continuous memory
	// slice from the pool
	if len(l.data) == 0 {
		l.data = memPool.Get(initialBufferSize)
	}

	// If required, grow the buffer
	if l.bufPos+l.elementSize >= len(l.data) {

		// If the buffer size is already at its limit, reject the new element
		if len(l.data) >= l.sizeLimit {
			return false
		}

		l.grow(min(l.sizeLimit, 2*len(l.data)))
	}

	// Transfer data to the buffer
	copy(l.data[l.bufPos:], ipLayer)
	l.data[l.bufPos+l.snapLen] = pktType
	*(*uint32)(unsafe.Pointer(&l.data[l.bufPos+l.snapLen+1])) = pktSize // #nosec G103

	// Increment buffer position
	l.bufPos += l.elementSize

	return true
}

// Get fetches the i-th element from the buffer (zero-copy)
func (l *LocalBuffer) Get(i int) (capture.IPLayer, byte, uint32) {
	return l.data[i*l.elementSize : i*l.elementSize+l.snapLen], l.data[i*l.elementSize+l.snapLen], *(*uint32)(unsafe.Pointer(&l.data[i*l.elementSize+l.snapLen+1])) // #nosec G103
}

// N returns the number of elements in the buffer
func (l *LocalBuffer) N() int {
	return l.bufPos / l.elementSize
}

// Reset puts the buffer into its initial state and returns any memory to the pool
func (l *LocalBuffer) Reset() {
	memPool.Put(l.data)
	l.bufPos = 0
	l.data = nil
}

///////////////////////////////////////////////////////////////////////////////////

func (l *LocalBuffer) grow(newSize int) {
	newData := make([]byte, newSize)
	copy(newData, l.data)
	l.data = newData
}

// TODO: With Go 1.21, this is is a built-in function!
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
