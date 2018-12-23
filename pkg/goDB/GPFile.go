package goDB

/*
#cgo CFLAGS: -O3
#define LZ4_STATIC_LINKING_ONLY

#include <stdlib.h>
#include <stdio.h>
#include "lz4/lz4hc.h"
#include "lz4/lz4.h"

int cCompress(int len, char *input, char *output, int level) {
  return LZ4_compressHC2(input, output, len, level);
}

int cUncompress(char *output, int out_len, char *input) {
  return LZ4_decompress_fast(input, output, out_len);
}
*/
import "C"

import (
	"errors"
	"os"
	"strconv"
	"unsafe"
)

const (
	BUF_SIZE = 4096         // 512 * 64bit
	N_ELEM   = BUF_SIZE / 8 // 512
)

type GPFile struct {
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
	cur_file *os.File
	w_buf    []byte

	last_seek_pos int64
}

type GPFiler interface {
	BlocksUsed() (int, error)
	WriteTimedBlock(timestamp int64, data []byte) error
	ReadTimedBlock(timestamp int64) ([]byte, error)
	ReadBlock(block int) ([]byte, error)
	GetBlocks() ([]int64, error)
	GetTimestamps() ([]int64, error)
	Close() error
}

func NewGPFile(p string) (*GPFile, error) {
	var (
		buf_h            = make([]byte, BUF_SIZE)
		buf_ts           = make([]byte, BUF_SIZE)
		buf_len          = make([]byte, BUF_SIZE)
		f                *os.File
		n_h, n_ts, n_len int
		err              error
	)

	// open file if it exists and read header, otherwise create it
	// and write empty header
	if _, err = os.Stat(p); err == nil {
		if f, err = os.Open(p); err != nil {
			return nil, err
		}
		if n_h, err = f.Read(buf_h); err != nil {
			return nil, err
		}
		if n_ts, err = f.Read(buf_ts); err != nil {
			return nil, err
		}
		if n_len, err = f.Read(buf_len); err != nil {
			return nil, err
		}
	} else {
		if f, err = os.Create(p); err != nil {
			return nil, err
		}
		if n_h, err = f.Write(buf_h); err != nil {
			return nil, err
		}
		if n_ts, err = f.Write(buf_ts); err != nil {
			return nil, err
		}
		if n_len, err = f.Write(buf_len); err != nil {
			return nil, err
		}
		f.Sync()
	}

	if n_h != BUF_SIZE {
		return nil, errors.New("Invalid header (blocks)")
	}
	if n_ts != BUF_SIZE {
		return nil, errors.New("Invalid header (lookup table)")
	}
	if n_len != BUF_SIZE {
		return nil, errors.New("Invalid header (block lengths)")
	}

	// read the header information
	var h = make([]int64, N_ELEM)
	var ts = make([]int64, N_ELEM)
	var le = make([]int64, N_ELEM)
	var pos int = 0
	for i := 0; i < N_ELEM; i++ {
		h[i] = int64(buf_h[pos])<<56 | int64(buf_h[pos+1])<<48 | int64(buf_h[pos+2])<<40 | int64(buf_h[pos+3])<<32 | int64(buf_h[pos+4])<<24 | int64(buf_h[pos+5])<<16 | int64(buf_h[pos+6])<<8 | int64(buf_h[pos+7])
		ts[i] = int64(buf_ts[pos])<<56 | int64(buf_ts[pos+1])<<48 | int64(buf_ts[pos+2])<<40 | int64(buf_ts[pos+3])<<32 | int64(buf_ts[pos+4])<<24 | int64(buf_ts[pos+5])<<16 | int64(buf_ts[pos+6])<<8 | int64(buf_ts[pos+7])
		le[i] = int64(buf_len[pos])<<56 | int64(buf_len[pos+1])<<48 | int64(buf_len[pos+2])<<40 | int64(buf_len[pos+3])<<32 | int64(buf_len[pos+4])<<24 | int64(buf_len[pos+5])<<16 | int64(buf_len[pos+6])<<8 | int64(buf_len[pos+7])
		pos += 8
	}

	return &GPFile{h, ts, le, p, f, make([]byte, BUF_SIZE*3), 0}, nil
}

func (f *GPFile) BlocksUsed() (int, error) {
	for i := 0; i < N_ELEM; i++ {
		if f.timestamps[i] == 0 && f.blocks[i] == 0 && f.lengths[i] == 0 {
			return i, nil
		}
	}
	return -1, errors.New("Could not retrieve number of allocated blocks")
}

func (f *GPFile) ReadBlock(block int) ([]byte, error) {
	if f.timestamps[block] == 0 && f.blocks[block] == 0 && f.lengths[block] == 0 {
		return nil, errors.New("Block " + strconv.Itoa(block) + " is empty")
	}

	var (
		err      error
		seek_pos int64 = BUF_SIZE * 3
		read_len int64
		n_read   int
	)

	// Check if file has already been opened for reading. If not, open it
	if f.cur_file == nil {
		if f.cur_file, err = os.OpenFile(f.filename, os.O_RDONLY, 0600); err != nil {
			return nil, err
		}
	}

	// If first block is requested, set seek position to end of header and read length of
	// first block. Otherwise start at last block's end
	read_len = f.blocks[block] - BUF_SIZE*3
	if block != 0 {
		seek_pos = f.blocks[block-1]
		read_len = f.blocks[block] - f.blocks[block-1]
	}

	// if the file is read continuously, do not seek
	if seek_pos != f.last_seek_pos {
		if _, err = f.cur_file.Seek(seek_pos, 0); err != nil {
			return nil, err
		}

		f.last_seek_pos = seek_pos
	}

	buf_comp := make([]byte, read_len)
	if n_read, err = f.cur_file.Read(buf_comp); err != nil {
		return nil, err
	}

	if int64(n_read) != read_len {
		return nil, errors.New("Incorrect number of bytes read from file")
	}

	buf := make([]byte, f.lengths[block])

	var uncomp_len int = int(C.cUncompress((*C.char)(unsafe.Pointer(&buf[0])), C.int(f.lengths[block]), (*C.char)(unsafe.Pointer(&buf_comp[0]))))

	if int64(uncomp_len) != read_len {
		return nil, errors.New("Incorrect number of bytes read for decompression")
	}

	return buf, nil
}

func (f *GPFile) ReadTimedBlock(timestamp int64) ([]byte, error) {
	for i := 0; i < N_ELEM; i++ {
		if f.timestamps[i] == timestamp {
			return f.ReadBlock(i)
		}
	}

	return nil, errors.New("Timestamp " + strconv.Itoa(int(timestamp)) + " not found")
}

func (f *GPFile) WriteTimedBlock(timestamp int64, data []byte, comp int) error {
	var (
		nextFreeBlock = int64(-1)
		cur_wfile     *os.File
		buf           []byte
		err           error
		n_write       int
		new_pos       int64
	)

	for new_pos = 0; new_pos < N_ELEM; new_pos++ {
		cur_timestamp := f.timestamps[new_pos]
		if cur_timestamp == timestamp {
			return errors.New("Timestamp" + strconv.Itoa(int(cur_timestamp)) + " already exists in file " + f.filename)
		} else if cur_timestamp == 0 {
			if new_pos != 0 {
				nextFreeBlock = f.blocks[new_pos-1]
			} else {
				nextFreeBlock = BUF_SIZE * 3
			}
			break
		}
	}

	if nextFreeBlock == -1 {
		return errors.New("File is full")
	}

	// LZ4 states that non-compressible data can be expanded to up to 0.4%.
	// This length bound is the conservative version of the bound specified in the LZ4 source
	buf = make([]byte, int((1.004*float64(len(data)))+16))
	var comp_len int = int(C.cCompress(C.int(len(data)), (*C.char)(unsafe.Pointer(&data[0])), (*C.char)(unsafe.Pointer(&buf[0])), C.int(comp)))

	if cur_wfile, err = os.OpenFile(f.filename, os.O_APPEND|os.O_WRONLY, 0600); err != nil {
		return err
	}

	// sanity check whether the computed worst case has been exceeded in C call
	if len(buf) < comp_len {
		return errors.New("Buffer size mismatch for compressed data")
	}

	if n_write, err = cur_wfile.Write(buf[0:comp_len]); err != nil {
		return err
	}

	cur_wfile.Close()

	// Update header
	f.blocks[new_pos] = nextFreeBlock + int64(n_write)
	f.timestamps[new_pos] = timestamp
	f.lengths[new_pos] = int64(len(data))

	var pos int = 0
	for i := 0; i < N_ELEM; i++ {
		for j := 0; j < 8; j++ {
			f.w_buf[pos+j] = byte(f.blocks[i] >> uint(56-(j*8)))
			f.w_buf[BUF_SIZE+pos+j] = byte(f.timestamps[i] >> uint(56-(j*8)))
			f.w_buf[BUF_SIZE+BUF_SIZE+pos+j] = byte(f.lengths[i] >> uint(56-(j*8)))
		}
		pos += 8
	}

	if cur_wfile, err = os.OpenFile(f.filename, os.O_WRONLY, 0600); err != nil {
		return err
	}
	if _, err = cur_wfile.Write(f.w_buf); err != nil {
		return err
	}

	cur_wfile.Close()

	return nil
}

func (f *GPFile) GetBlocks() []int64 {
	return f.blocks
}

func (f *GPFile) GetTimestamps() []int64 {
	return f.timestamps
}

func (f *GPFile) Close() error {
	if f.cur_file != nil {
		return f.cur_file.Close()
	}
	return nil
}
