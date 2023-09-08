package capture

import (
	"unsafe"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/fako1024/gotools/concurrency"
	"golang.org/x/sys/unix"
)

const (

	// bufElementAddSize denotes the required size for a buffer element
	// (size of EPHash + 4 bytes for pktSize + 1 byte for pktType, isIPv4, auxInfo, errno, respectively)
	bufElementSize = capturetypes.EPHashSize + 8
)

var (

	// Initial size of a buffer
	initialBufferSize = unix.Getpagesize()

	// Global (limited) memory pool used to minimize allocations
	memPool       = concurrency.NewMemPool(config.DefaultLocalBufferNumBuffers)
	maxBufferSize = config.DefaultLocalBufferSizeLimit
)

// LocalBuffer denotes a local packet buffer used to temporarily capture packets
// from the source (e.g. during rotation) to avoid a ring / kernel buffer overflow
type LocalBuffer struct {
	data   []byte // continuous buffer slice
	bufPos int    // current position in buffer slice
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
func (l *LocalBuffer) Add(epHash capturetypes.EPHash, pktType byte, pktSize uint32, isIPv4 bool, auxInfo byte, errno capturetypes.ParsingErrno) (ok bool) {

	// Ascertain the current size of the underlying data slice (from the memory pool)
	// and grow if required
	if len(l.data) == 0 {
		l.data = make([]byte, initialBufferSize)
	}

	// If required, attempt to grow the buffer
	if l.bufPos+bufElementSize >= len(l.data) {

		// If the buffer size is already at its limit, reject the new element
		if len(l.data) >= maxBufferSize {
			return false
		}

		l.grow(min(maxBufferSize, 2*len(l.data)))
	}

	// Transfer data to the buffer
	copy(l.data[l.bufPos:], epHash[:])
	l.data[l.bufPos+capturetypes.EPHashSize] = pktType
	if isIPv4 {
		l.data[l.bufPos+capturetypes.EPHashSize+1] = 0
	} else {
		l.data[l.bufPos+capturetypes.EPHashSize+1] = 1
	}
	l.data[l.bufPos+capturetypes.EPHashSize+2] = auxInfo
	*(*int8)(unsafe.Pointer(&l.data[l.bufPos+capturetypes.EPHashSize+3])) = int8(errno) // #nosec G103
	*(*uint32)(unsafe.Pointer(&l.data[l.bufPos+capturetypes.EPHashSize+4])) = pktSize   // #nosec G103

	// Increment buffer position
	l.bufPos += bufElementSize

	return true
}

// Get fetches the i-th element from the buffer
func (l *LocalBuffer) Get(i int) (epHash capturetypes.EPHash, pktType byte, pktSize uint32, isIPv4 bool, auxInfo byte, errno capturetypes.ParsingErrno) {
	return capturetypes.EPHash(l.data[i*bufElementSize : i*bufElementSize+capturetypes.EPHashSize]),
		l.data[i*bufElementSize+capturetypes.EPHashSize],
		*(*uint32)(unsafe.Pointer(&l.data[i*bufElementSize+capturetypes.EPHashSize+4])),
		l.data[i*bufElementSize+capturetypes.EPHashSize+1] > 0,
		l.data[i*bufElementSize+capturetypes.EPHashSize+2],
		capturetypes.ParsingErrno(*(*int8)(unsafe.Pointer(&l.data[i*bufElementSize+capturetypes.EPHashSize+3]))) // #nosec G103
}

// N returns the number of elements in the buffer
func (l *LocalBuffer) N() int {
	return l.bufPos / bufElementSize
}

///////////////////////////////////////////////////////////////////////////////////

// setLocalBuffers sets the number of (and hence the maximum concurrency for Status() calls) and
// maximum size of the local memory buffers (globally, not per interface)
func setLocalBuffers(nBuffers, sizeLimit int) {
	if memPool != nil {
		memPool.Clear()
	}
	memPool = concurrency.NewMemPool(nBuffers)
	maxBufferSize = sizeLimit
}

func (l *LocalBuffer) grow(newSize int) {
	newData := make([]byte, newSize)
	copy(newData, l.data)
	l.data = newData
}
