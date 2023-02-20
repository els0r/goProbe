package gpfile

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"
	"time"
)

type ReadWriteSeekCloser interface {
	Stat() (fs.FileInfo, error)

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
func NewMemFile(r ReadWriteSeekCloser, pool *MemPool) (*MemFile, error) {
	obj := MemFile{
		data: pool.Get(),
		pool: pool,
	}
	stat, err := r.Stat()
	if err != nil {
		return nil, err
	}
	if cap(obj.data) < int(stat.Size()) {
		obj.data = make([]byte, stat.Size())
	}
	obj.data = obj.data[:stat.Size()]
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

// Data provides zero-copy access to the underlying data of the MemFile
func (m *MemFile) Data() []byte {
	return m.data
}

// Close fulfils the underlying io.Closer interface (returning the buffer to the pool)
func (m *MemFile) Close() error {
	m.pool.Put(m.data)
	return nil
}

// Stat return the (stub) Stat element providing the length of the underlying data
func (m *MemFile) Stat() (fs.FileInfo, error) {
	return &memStat{
		size: int64(len(m.data)),
	}, nil
}

// A memStat is the (stub) implementation of FileInfo returned by Stat and Lstat, basically
// only providing the ability to obtain the size / length of the underlying data
type memStat struct {
	size int64
}

func (s *memStat) Size() int64        { return s.size }
func (s *memStat) Mode() os.FileMode  { return 0 }
func (s *memStat) ModTime() time.Time { return time.Unix(0, 0) }
func (s *memStat) IsDir() bool        { return false }
func (s *memStat) Name() string       { return "" }
func (s *memStat) Sys() any           { return nil }
