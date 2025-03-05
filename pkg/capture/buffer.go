package capture

import (
	"unsafe"

	"github.com/els0r/goProbe/v4/pkg/capture/capturetypes"
	"github.com/fako1024/gotools/concurrency"
	"golang.org/x/sys/unix"
)

const (

	// bufElementAddSize denotes the required (additional) size for a buffer element
	// (size of EPHash + 4 bytes for pktSize + 1 byte for pktType, isIPv4, errno, respectively)
	bufElementAddSize = 7
)

var (

	// Initial size of a buffer
	initialBufferSize = unix.Getpagesize()
)

// LocalBufferPool provides a wrapper around a MemPoolLimitUnique with a maximum size
type LocalBufferPool struct {
	NBuffers      int
	MaxBufferSize int
	*concurrency.MemPoolLimitUnique
}

// NewLocalBufferPool initializes a new local buffer pool
func NewLocalBufferPool(nBuffers, maxBufferSize int) *LocalBufferPool {
	return &LocalBufferPool{
		NBuffers:           nBuffers,
		MaxBufferSize:      maxBufferSize,
		MemPoolLimitUnique: concurrency.NewMemPoolLimitUnique(nBuffers, initialBufferSize),
	}
}

// LocalBuffer denotes a local packet buffer used to temporarily capture packets
// from the source (e.g. during rotation) to avoid a ring / kernel buffer overflow
type LocalBuffer struct {
	data        []byte // continuous buffer slice
	writeBufPos int    // current position in buffer slice
	readBufPos  int    // current position in buffer slice

	memPool *LocalBufferPool
}

// NewLocalBuffer initializes a new local buffer using a global memory pool and a maximum buffer size
func NewLocalBuffer(memPool *LocalBufferPool) *LocalBuffer {
	return &LocalBuffer{
		memPool: memPool,
	}
}

// Assign sets the actual underlying data slice (obtained from a memory pool) of this buffer
func (l *LocalBuffer) Assign(data []byte) {
	l.data = data

	// Ascertain the current size of the underlying data slice and grow if required
	if len(l.data) < initialBufferSize {
		l.data = l.memPool.Resize(l.data, initialBufferSize)
	}
}

// Reset resets the buffer position
func (l *LocalBuffer) Reset() {
	l.writeBufPos = 0
	l.readBufPos = 0
}

// Usage return the relative fraction of the buffer capacity in use (i.e. written to, independent of
// number of items already retreived by Next())
func (l *LocalBuffer) Usage() float64 {

	// Note: maxBufferSize is guarded against zero in setLocalBuffers(), so this cannot cause division by zero
	return float64(l.writeBufPos) / float64(l.memPool.MaxBufferSize)
}

// Add adds an element to the buffer, returning ok = true if successful
// If the buffer is full / may not grow any further, ok is false
func (l *LocalBuffer) Add(epHash []byte, pktType byte, pktSize uint32, isIPv4 bool, auxInfo byte, errno capturetypes.ParsingErrno) (ok bool) {

	// If required, attempt to grow the buffer
	if l.writeBufPos+len(epHash)+bufElementAddSize >= len(l.data) {

		// If the buffer size is already at its limit, reject the new element
		if len(l.data) >= l.memPool.MaxBufferSize {
			return false
		}

		l.grow(min(l.memPool.MaxBufferSize, 2*len(l.data)))
	}

	// Transfer data to the buffer
	if isIPv4 {
		l.data[l.writeBufPos] = 0
		copy(l.data[l.writeBufPos+1:l.writeBufPos+capturetypes.EPHashSizeV4+1], epHash)

		l.data[l.writeBufPos+capturetypes.EPHashSizeV4+1] = pktType
		l.data[l.writeBufPos+capturetypes.EPHashSizeV4+2] = auxInfo
		*(*int8)(unsafe.Pointer(&l.data[l.writeBufPos+capturetypes.EPHashSizeV4+3])) = int8(errno) // #nosec G103
		*(*uint32)(unsafe.Pointer(&l.data[l.writeBufPos+capturetypes.EPHashSizeV4+4])) = pktSize   // #nosec G103

		// Increment buffer position
		l.writeBufPos += capturetypes.EPHashSizeV4 + bufElementAddSize

		return true
	}

	l.data[l.writeBufPos] = 1
	copy(l.data[l.writeBufPos+1:l.writeBufPos+capturetypes.EPHashSizeV6+1], epHash)

	l.data[l.writeBufPos+capturetypes.EPHashSizeV6+1] = pktType
	l.data[l.writeBufPos+capturetypes.EPHashSizeV6+2] = auxInfo
	*(*int8)(unsafe.Pointer(&l.data[l.writeBufPos+capturetypes.EPHashSizeV6+3])) = int8(errno) // #nosec G103
	*(*uint32)(unsafe.Pointer(&l.data[l.writeBufPos+capturetypes.EPHashSizeV6+4])) = pktSize   // #nosec G103

	// Increment buffer position
	l.writeBufPos += capturetypes.EPHashSizeV6 + bufElementAddSize

	return true
}

// Next fetches the i-th element from the buffer
func (l *LocalBuffer) Next() ([]byte, byte, uint32, bool, byte, capturetypes.ParsingErrno, bool) {

	if l.readBufPos >= l.writeBufPos {
		return nil, 0, 0, false, 0, 0, false
	}

	pos := l.readBufPos
	if l.data[pos] == 0 {
		l.readBufPos += capturetypes.EPHashSizeV4 + bufElementAddSize
		return l.data[pos+1 : pos+1+capturetypes.EPHashSizeV4],
			l.data[pos+1+capturetypes.EPHashSizeV4],
			*(*uint32)(unsafe.Pointer(&l.data[pos+capturetypes.EPHashSizeV4+4])), // #nosec G103
			true,
			l.data[pos+capturetypes.EPHashSizeV4+2],
			capturetypes.ParsingErrno(*(*int8)(unsafe.Pointer(&l.data[pos+capturetypes.EPHashSizeV4+3]))), // #nosec G103
			true
	}

	l.readBufPos += capturetypes.EPHashSizeV6 + bufElementAddSize
	return l.data[pos+1 : pos+1+capturetypes.EPHashSizeV6],
		l.data[pos+1+capturetypes.EPHashSizeV6],
		*(*uint32)(unsafe.Pointer(&l.data[pos+capturetypes.EPHashSizeV6+4])), // #nosec G103
		false,
		l.data[pos+capturetypes.EPHashSizeV6+2],
		capturetypes.ParsingErrno(*(*int8)(unsafe.Pointer(&l.data[pos+capturetypes.EPHashSizeV6+3]))), // #nosec G103
		true
}

func (l *LocalBuffer) grow(newSize int) {
	l.data = l.memPool.Resize(l.data, newSize)
}
