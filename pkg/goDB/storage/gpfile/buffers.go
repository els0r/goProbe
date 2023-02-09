package gpfile

import (
	"fmt"
	"io"
	"sync"
)

type readWriteSeekCloser interface {
	io.Reader
	io.Writer
	io.Seeker
	io.Closer
}

// MemPool wraps a standard sync.Pool
type MemPool struct {
	sync.Pool
}

// NewMemPool instantiates a new memory pool that manages bytes slices
// of a given capacity
func NewMemPool() *MemPool {
	return &MemPool{
		Pool: sync.Pool{
			New: func() any {
				return make([]byte, 0)
			},
		},
	}
}

// Get retrieves a memory element (already performing the type assertion)
func (p *MemPool) Get() []byte {
	return p.Pool.Get().([]byte)
}

// Put returns a memory element to the pool, resetting its size to capacity
// in the process
func (p *MemPool) Put(elem []byte) {
	elem = elem[:cap(elem)]

	// nolint:staticcheck
	p.Pool.Put(elem)
}

// MemFile denotes an in-memory abstraction of an underlying file, acting as
// a buffer (drawing memory from a pool)
type MemFile struct {
	data []byte
	pos  int

	pool *MemPool
}

// NewMemFile instantiates a new in-memory file buffer
func NewMemFile(r io.ReadCloser, l int, pool *MemPool) (*MemFile, error) {
	obj := MemFile{
		data: pool.Get(),
		pool: pool,
	}
	if len(obj.data) < l {
		obj.data = make([]byte, l)
	}
	obj.data = obj.data[:l]
	if _, err := io.ReadFull(r, obj.data); err != nil {
		return nil, err
	}
	return &obj, r.Close()
}

// Read fulfils the io.Reader interface (reading len(p) bytes from the buffer)
func (m *MemFile) Read(p []byte) (n int, err error) {
	n = copy(p, m.data[m.pos:])
	if n != len(p) {
		return n, fmt.Errorf("unexpected number of bytes read (want %d, have %d)", len(p), n)
	}
	m.pos += n
	return
}

// Write fulfils the io.Writer interface (writing len(p) bytes to the buffer)
func (m *MemFile) Write(p []byte) (n int, err error) {
	n = copy(m.data[m.pos:], p)
	if n != len(p) {
		return n, fmt.Errorf("unexpected number of bytes written (want %d, have %d)", len(p), n)
	}
	m.pos += n
	return
}

// Seek fulfils the io.Seeker interface (seeking to a designated position)
func (m *MemFile) Seek(offset int64, whence int) (int64, error) {
	if whence != 0 {
		panic("only supports seek from start of buffer")
	}
	if int(offset) >= len(m.data) {
		return 0, io.EOF
	}
	m.pos = int(offset)
	return int64(m.pos), nil
}

// Close fulfils the underlying io.Closer interface (returning the buffer to the pool)
func (m *MemFile) Close() error {
	m.pool.Put(m.data)
	return nil
}
