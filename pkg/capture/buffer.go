package capture

import (
	"unsafe"

	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/fako1024/slimcap/capture"
	"golang.org/x/sys/unix"
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
	data []byte

	sizeLimit   int
	snapLen     int
	elementSize int

	bufPos int
}

// NewLocalBuffer instantiates a new buffer
func NewLocalBuffer(captureHandle capture.SourceZeroCopy, sizeLimit int) *LocalBuffer {
	p := captureHandle.NewPacket()
	return &LocalBuffer{
		snapLen:     len(p.IPLayer()),
		elementSize: len(p.IPLayer()) + 5, // snaplen + sizes for pktType and pktSize
		sizeLimit:   sizeLimit,
	}
}

// Add adds an element to the buffer
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
