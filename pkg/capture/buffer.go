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

	// Global (limited) memory pool used to minimize allocations
	memPool       = gpfile.NewMemPool(config.DefaultLocalBufferNumBuffers)
	maxBufferSize = config.DefaultLocalBufferSizeLimit
)

// LocalBuffer denotes a local packet buffer used to temporarily capture packets
// from the source (e.g. during rotation) to avoid a ring / kernel buffer overflow
type LocalBuffer struct {
	data []byte // continuous buffer slice

	snapLen     int // capture length / snaplen for the underlying packet source
	elementSize int // size of an individual element stored in the buffer

	bufPos int // current position in continuous buffer slice
}

// NewLocalBuffer instantiates a new buffer
func NewLocalBuffer(captureHandle capture.SourceZeroCopy) *LocalBuffer {
	p := captureHandle.NewPacket()
	return &LocalBuffer{
		snapLen:     len(p.IPLayer()),
		elementSize: len(p.IPLayer()) + bufElementAddSize, // snaplen + sizes for pktType and pktSize
	}
}

// Assign sets the actual underlying data slice (obtained from a memory pool) of this buffer
func (l *LocalBuffer) Assign(data []byte) {
	l.data = data
}

// Release returns the data slice to the memory pool and resets the buffer position
func (l *LocalBuffer) Release() {
	memPool.Put(l.data)
	l.bufPos = 0
	l.data = nil
}

// Add adds an element to the buffer, returning ok = true if successful
// If the buffer is full / may not grow any further, ok is false
func (l *LocalBuffer) Add(ipLayer capture.IPLayer, pktType byte, pktSize uint32) (ok bool) {

	// Ascertain the current size of the underlying data slice (from the memory pool)
	// and grow if required
	if len(l.data) == 0 {
		l.data = make([]byte, initialBufferSize)
	}

	// If required, grow the buffer
	if l.bufPos+l.elementSize >= len(l.data) {

		// If the buffer size is already at its limit, reject the new element
		if len(l.data) >= maxBufferSize {
			return false
		}

		l.grow(min(maxBufferSize, 2*len(l.data)))
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

///////////////////////////////////////////////////////////////////////////////////

// setLocalBuffers sets the number of (and hence the maximum concurrency for Status() calls) and
// maximum size of the local memory buffers (globally, not per interface)
func setLocalBuffers(nBuffers, sizeLimit int) {
	if memPool != nil {
		memPool.Clear()
	}
	memPool = gpfile.NewMemPool(nBuffers)
	maxBufferSize = sizeLimit
}

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
