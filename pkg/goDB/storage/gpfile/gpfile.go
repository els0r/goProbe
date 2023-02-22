package gpfile

import (
	"bufio"
	"fmt"
	"os"

	"github.com/els0r/goProbe/pkg/goDB/encoder"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage"
)

const (
	// FileSuffix denotes the suffix used for the raw data stored
	FileSuffix = ".gpf"

	// defaultPermissions denotes the permissions used for file creation
	defaultPermissions = 0644

	// defaultEncoderType denotes the default encoder / compressor
	defaultEncoderType = encoders.EncoderTypeLZ4

	// headerVersion denotes the current header version
	headerVersion = 1

	// ModeRead denotes read access
	ModeRead = os.O_RDONLY

	// ModeWrite denotes write / append access
	ModeWrite = os.O_APPEND | os.O_CREATE | os.O_WRONLY
)

// GPFile implements the binary data file used to store goProbe's flows
type GPFile struct {

	// filename denotes the path to the GPF file
	filename string

	// file denotes the pointer to the data file
	file            ReadWriteSeekCloser
	fileWriteBuffer *bufio.Writer

	// header denotes the block header (list of blocks) contained in this file
	header *storage.BlockHeader

	// Current / last seek position in file for read operation, used for optimized
	// sequential read
	lastSeekPos int64

	// defaultEncoderType governs how data blocks are (de-)compressed by default
	defaultEncoderType encoders.Type
	defaultEncoder     encoder.Encoder
	nullEncoder        encoder.Encoder

	// accessMode denotes if the file is opened for read or write operations (to avoid
	// race conditions and unpredictable behavior, only one mode is possible at a time)
	accessMode int

	// Reusable buffers for compression / decompression
	uncompData, blockData []byte

	// Memory pool (optional)
	memPool *MemPool
}

// New returns a new GPFile object to read and write goProbe flow data
func New(filename string, header *storage.BlockHeader, accessMode int, options ...Option) (*GPFile, error) {
	g := &GPFile{
		filename:           filename,
		header:             header,
		accessMode:         accessMode,
		defaultEncoderType: defaultEncoderType,
	}

	if header == nil {
		return nil, fmt.Errorf("header information missing when trying to access `%s`", filename)
	}

	// apply functional options
	for _, opt := range options {
		opt(g)
	}

	// Initialize default encoder based on requested encoder type. If the colunn has
	// a high-cardinality, the default encoder is overwritten with the high cardinality
	// encoder
	var err error
	if g.defaultEncoder, err = encoder.New(g.defaultEncoderType); err != nil {
		return nil, err
	}
	if g.nullEncoder, err = encoder.New(encoders.EncoderTypeNull); err != nil {
		return nil, err
	}

	return g, nil
}

// Blocks return the list of available blocks (and its metadata)
func (g *GPFile) Blocks() (*storage.BlockHeader, error) {
	return g.header, nil
}

// ReadBlock searches if a block for a given timestamp exists and returns in its data
func (g *GPFile) ReadBlock(timestamp int64) ([]byte, error) {

	// Check if the requested block exists
	blockIdx, found := g.header.BlockIndex(timestamp)
	if !found {
		return nil, fmt.Errorf("block for timestamp %v not found", timestamp)
	}

	return g.ReadBlockAtIndex(blockIdx)
}

// ReadBlockAtIndex returns the data of the indexed block
func (g *GPFile) ReadBlockAtIndex(idx int) ([]byte, error) {

	// Check that the file has been opened in the correct mode
	if g.accessMode != ModeRead {
		return nil, fmt.Errorf("cannot read from GPFile in write mode")
	}
	block := g.header.BlockList[idx]

	// If there is no data to be expected, return
	if block.RawLen == 0 {
		return []byte{}, nil
	}

	// If the data file is not yet available, open it
	if g.file == nil {
		if err := g.open(g.accessMode); err != nil {
			return nil, err
		}
	}

	// if the file is read continuously, do not seek
	var (
		seekPos = block.Offset
		err     error
	)
	if seekPos != g.lastSeekPos {
		if g.lastSeekPos, err = g.file.Seek(seekPos, 0); err != nil {
			return nil, err
		}
	}

	// Perform decompression of data and store in output slice
	var nRead int
	if cap(g.uncompData) < block.RawLen {
		g.uncompData = make([]byte, 0, 2*block.RawLen)
	}
	g.uncompData = g.uncompData[:block.RawLen]
	if block.EncoderType != encoders.EncoderTypeNull {

		// Instantiate decoder / decompressor (if required)
		if block.EncoderType != g.defaultEncoder.Type() {
			decoder, err := encoder.New(block.EncoderType)
			if err != nil {
				return nil, fmt.Errorf("failed to decode block %d based on detected encoder type %v: %w", block, block.EncoderType, err)
			}
			g.defaultEncoder = decoder
		}

		if cap(g.blockData) < block.Len {
			g.blockData = make([]byte, 0, 2*block.Len)
		}
		g.blockData = g.blockData[:block.Len]
		nRead, err = g.defaultEncoder.Decompress(g.blockData, g.uncompData, g.file)
	} else {
		// micro-optimization that saves the allocation of blockData for decompression
		// in the Null decompression case, since it is essentially just a byte read
		// and the src bytes aren't used
		nRead, err = g.nullEncoder.Decompress(nil, g.uncompData, g.file)
	}
	if err != nil {
		return nil, err
	}
	if nRead != block.RawLen {
		return nil, fmt.Errorf("unexpected amount of bytes after decompression, want %d, have %d", block.RawLen, nRead)
	}
	g.lastSeekPos += int64(block.Len)

	return g.uncompData, nil
}

// WriteBlock writes data for a given timestamp to the file
func (g *GPFile) WriteBlock(timestamp int64, blockData []byte) error {
	blockIdx, exists := g.header.BlockIndex(timestamp)
	if exists {
		return fmt.Errorf("timestamp %d already present: offset=%d", timestamp, g.header.BlockList[int64(blockIdx)].Offset)
	}

	// Check that the file has been opened in the correct mode
	if g.accessMode != ModeWrite {
		return fmt.Errorf("cannot write to GPFile in read mode")
	}

	// If block data is empty, do nothing except updating the header
	if len(blockData) == 0 {
		block := storage.Block{
			Offset:      g.header.CurrentOffset,
			EncoderType: encoders.EncoderTypeNull,
		}
		g.header.AddBlock(timestamp, block)
		return nil
	}

	// If the data file is not yet available, open it
	if g.file == nil {
		if err := g.open(g.accessMode); err != nil {
			return err
		}
	}

	// Compress + write block data to file (append)
	nWritten, err := g.defaultEncoder.Compress(blockData, g.blockData, g.fileWriteBuffer)
	if err != nil {
		return err
	}
	encType := g.defaultEncoderType

	// if compressed size is bigger than input size, make sure to rewrite the bytes
	// with NullCompression to optimize storage and increase read speed
	if nWritten > len(blockData) {
		encType = encoders.EncoderTypeNull
		g.fileWriteBuffer.Reset(g.file)
		nWritten, err = g.nullEncoder.Compress(blockData, g.blockData, g.fileWriteBuffer)
		if err != nil {
			return fmt.Errorf("failed to re-encode with %s encoder: %w", encType, err)
		}
	}
	if err = g.fileWriteBuffer.Flush(); err != nil {
		return err
	}

	// Update and write header data
	g.header.AddBlock(timestamp, storage.Block{
		Offset:      g.header.CurrentOffset,
		Len:         nWritten,
		RawLen:      len(blockData),
		EncoderType: encType,
	})
	g.header.CurrentOffset += int64(nWritten)

	return nil
}

func (g *GPFile) RawFile() ReadWriteSeekCloser {
	return g.file
}

// Close closes the file
func (g *GPFile) Close() error {
	if g.file != nil {
		return g.file.Close()
	}
	return nil
}

// Delete removes the file and its metadata
func (g *GPFile) Delete() error {
	return os.Remove(g.filename)
}

// Filename exposes the location of the GPF file
func (g *GPFile) Filename() string {
	return g.filename
}

// DefaultEncoder exposes the default encoding of the GPF file
func (g *GPFile) DefaultEncoder() encoder.Encoder {
	return g.defaultEncoder
}

////////////////////////////////////////////////////////////////////////////////

func (g *GPFile) open(flags int) (err error) {
	if g.file != nil {
		return fmt.Errorf("file %s is already open", g.filename)
	}

	// Open file for append, create if not exists
	g.file, err = os.OpenFile(g.filename, flags, defaultPermissions)
	if flags == ModeWrite {
		g.fileWriteBuffer = bufio.NewWriter(g.file)
	}
	if flags == ModeRead && g.memPool != nil {
		if g.file, err = NewMemFile(g.file, g.memPool); err != nil {
			return err
		}
	}

	return
}
