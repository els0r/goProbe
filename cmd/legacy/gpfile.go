package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/els0r/goProbe/pkg/goDB/encoder"
	"github.com/els0r/goProbe/pkg/goDB/encoder/lz4cust"
)

const (
	// BufSize allocates space for the header (512 slots for 64bit integers)
	BufSize = 4096
	// NumElements is the number of available header slots
	NumElements = BufSize / 8 // 512
)

// LegacyGPFile implements the binary data file used to store goProbe's flows
type LegacyGPFile struct {
	// The file header //
	// Contains 512 64 bit addresses pointing to the end
	// (+1 byte) of each compressed block and the lookup
	// table which stores 512 timestamps as int64 for
	// lookup without having to parse the file
	blocks     []int64
	timestamps []int64
	lengths    []int64

	// The path to the file
	filename string
	curFile  *os.File
	wBuf     []byte

	lastSeekPos int64

	// governs how data blocks are (de-)compressed
	encoder encoder.Encoder
}

// NewLegacyGPFile returns a new LegacyGPFile object to read and write goProbe flow data
func NewLegacyGPFile(p string) (*LegacyGPFile, error) {
	var (
		bufH          = make([]byte, BufSize)
		bufTS         = make([]byte, BufSize)
		bufLen        = make([]byte, BufSize)
		f             *os.File
		nH, nTS, nLen int
		err           error
	)

	// open file if it exists and read header
	if f, err = os.Open(filepath.Clean(p)); err != nil {
		return nil, err
	}
	if nH, err = f.Read(bufH); err != nil {
		return nil, err
	}
	if nTS, err = f.Read(bufTS); err != nil {
		return nil, err
	}
	if nLen, err = f.Read(bufLen); err != nil {
		return nil, err
	}
	if nH != BufSize {
		return nil, errors.New("invalid header (blocks)")
	}
	if nTS != BufSize {
		return nil, errors.New("invalid header (lookup table)")
	}
	if nLen != BufSize {
		return nil, errors.New("invalid header (block lengths)")
	}

	// read the header information
	var h = make([]int64, NumElements)
	var ts = make([]int64, NumElements)
	var le = make([]int64, NumElements)
	var pos int
	for i := 0; i < NumElements; i++ {
		h[i] = int64(bufH[pos])<<56 | int64(bufH[pos+1])<<48 | int64(bufH[pos+2])<<40 | int64(bufH[pos+3])<<32 | int64(bufH[pos+4])<<24 | int64(bufH[pos+5])<<16 | int64(bufH[pos+6])<<8 | int64(bufH[pos+7])
		ts[i] = int64(bufTS[pos])<<56 | int64(bufTS[pos+1])<<48 | int64(bufTS[pos+2])<<40 | int64(bufTS[pos+3])<<32 | int64(bufTS[pos+4])<<24 | int64(bufTS[pos+5])<<16 | int64(bufTS[pos+6])<<8 | int64(bufTS[pos+7])
		le[i] = int64(bufLen[pos])<<56 | int64(bufLen[pos+1])<<48 | int64(bufLen[pos+2])<<40 | int64(bufLen[pos+3])<<32 | int64(bufLen[pos+4])<<24 | int64(bufLen[pos+5])<<16 | int64(bufLen[pos+6])<<8 | int64(bufLen[pos+7])
		pos += 8
	}

	// the GP File uses (custom) LZ4 data block compression by default
	gpf := &LegacyGPFile{h, ts, le, p, f, make([]byte, BufSize*3), 0, lz4cust.New()}

	return gpf, nil
}

// ReadBlock returns the data for a given block in the file
func (f *LegacyGPFile) ReadBlock(block int) ([]byte, error) {
	if f.timestamps[block] == 0 && f.blocks[block] == 0 && f.lengths[block] == 0 {
		return nil, fmt.Errorf("block %d is empty", block)
	}

	var (
		err     error
		seekPos int64 = BufSize * 3
		readLen int64
	)

	// Check if file has already been opened for reading. If not, open it
	if f.curFile == nil {
		if f.curFile, err = os.OpenFile(f.filename, os.O_RDONLY, 0600); err != nil {
			return nil, err
		}
	}

	// If first block is requested, set seek position to end of header and read length of
	// first block. Otherwise start at last block's end
	readLen = f.blocks[block] - BufSize*3
	if block != 0 {
		seekPos = f.blocks[block-1]
		readLen = f.blocks[block] - f.blocks[block-1]
	}
	if readLen == 0 || f.lengths[block] == 0 {
		return []byte{}, nil
	}

	// if the file is read continuously, do not seek
	if seekPos != f.lastSeekPos {
		if f.lastSeekPos, err = f.curFile.Seek(seekPos, 0); err != nil {
			return nil, err
		}
	}

	// prepare data slices for decompression
	var (
		uncompLen int
		bufComp   = make([]byte, readLen)
		buf       = make([]byte, f.lengths[block])
	)

	uncompLen, err = f.encoder.Decompress(bufComp, buf, f.curFile)
	if err != nil {
		return nil, err
	}
	if int64(uncompLen) != f.lengths[block] {
		return nil, errors.New("incorrect number of bytes read for decompression")
	}
	f.lastSeekPos += readLen

	return buf, nil
}

// ReadTimedBlock searches if a block for a given timestamp exists and returns in its data
func (f *LegacyGPFile) ReadTimedBlock(timestamp int64) ([]byte, error) {
	for i := 0; i < NumElements; i++ {
		if f.timestamps[i] == timestamp {
			return f.ReadBlock(i)
		}
	}

	return nil, fmt.Errorf("timestamp %d not found", timestamp)
}

// GetBlocks returns the in-file location for all data blocks
func (f *LegacyGPFile) GetBlocks() []int64 {
	return f.blocks
}

// GetTimestamps returns all timestamps under which data blocks were stored
func (f *LegacyGPFile) GetTimestamps() []int64 {
	return f.timestamps
}

// Close closes the underlying file
func (f *LegacyGPFile) Close() error {
	if f.curFile != nil {
		return f.curFile.Close()
	}
	return nil
}
