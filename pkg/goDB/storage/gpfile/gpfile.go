package gpfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/els0r/goProbe/pkg/goDB/encoder"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage"
)

const (

	// HeaderFileSuffix denotes the suffix used for the header data
	HeaderFileSuffix = ".meta"

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
	file       *os.File
	fileBuffer *bufio.Writer

	// header denotes the block header (list of blocks) contained in this file
	header storage.BlockHeader

	// Current / last seek position in file for read operation, used for optimized
	// sequential read
	lastSeekPos int64

	// defaultEncoderType governs how data blocks are (de-)compressed by default
	defaultEncoderType encoders.Type
	defaultEncoder     encoder.Encoder

	// accessMode denotes if the file is opened for read or write operations (to avoid
	// race conditions and unpredictable behavior, only one mode is possible at a time)
	accessMode int
}

// New returns a new GPFile object to read and write goProbe flow data
func New(filename string, accessMode int, options ...Option) (*GPFile, error) {

	g := &GPFile{
		filename:           filename,
		accessMode:         accessMode,
		defaultEncoderType: defaultEncoderType,
	}

	// apply functional options
	for _, opt := range options {
		opt(g)
	}

	// Initialize default encoder based on requested encoder type
	var err error
	if g.defaultEncoder, err = encoder.New(g.defaultEncoderType); err != nil {
		return nil, err
	}

	// Read header if present
	if err = g.readHeader(); err != nil {
		return nil, err
	}

	return g, nil
}

// Blocks return the list of available blocks (and its metadata)
func (g *GPFile) Blocks() (storage.BlockHeader, error) {
	return g.header, nil
}

// ReadBlock searches if a block for a given timestamp exists and returns in its data
func (g *GPFile) ReadBlock(timestamp int64) ([]byte, error) {

	// Check that the file has been opened in the correct mode
	if g.accessMode != ModeRead {
		return nil, fmt.Errorf("Cannot read from GPFile in write mode")
	}

	// Check if the requested block exists
	block, found := g.header.Blocks[timestamp]
	if !found {
		return nil, fmt.Errorf("Block for timestamp %v not found", timestamp)
	}

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

	// Instantiate decoder / decompressor
	if block.EncoderType != g.defaultEncoder.Type() {
		decoder, err := encoder.New(block.EncoderType)
		if err != nil {
			return nil, fmt.Errorf("Failed to decode block %d based on detected encoder type %v: %s", block, block.EncoderType, err)
		}
		g.defaultEncoder = decoder
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
	blockData := make([]byte, block.Len)
	uncompData := make([]byte, block.RawLen)
	nRead, err := g.defaultEncoder.Decompress(blockData, uncompData, g.file)
	if err != nil {
		return nil, err
	}
	if nRead != block.RawLen {
		return nil, fmt.Errorf("Unexpected amount of bytes after decompression, want %d, have %d", block.RawLen, nRead)
	}
	g.lastSeekPos += int64(block.Len)

	return uncompData, nil
}

// WriteBlock writes data for a given timestamp to the file
func (g *GPFile) WriteBlock(timestamp int64, blockData []byte) error {

	// Check that the file has been opened in the correct mode
	if g.accessMode != ModeWrite {
		return fmt.Errorf("Cannot write to GPFile in read mode")
	}

	// If block data is empty, do nothing except updating the header
	if len(blockData) == 0 {
		g.header.Blocks[timestamp] = storage.Block{
			Offset:      g.header.CurrentOffset,
			EncoderType: g.defaultEncoderType,
		}
		return g.writeHeader()
	}

	// If the data file is not yet available, open it
	if g.file == nil {
		if err := g.open(g.accessMode); err != nil {
			return err
		}
	}

	// Compress + write block data to file (append)
	nWritten, err := g.defaultEncoder.Compress(blockData, g.fileBuffer)
	if err != nil {
		return err
	}
	if err = g.fileBuffer.Flush(); err != nil {
		return err
	}

	// Update and write header data
	g.header.Blocks[timestamp] = storage.Block{
		Offset:      g.header.CurrentOffset,
		Len:         nWritten,
		RawLen:      len(blockData),
		EncoderType: g.defaultEncoderType,
	}
	g.header.CurrentOffset += int64(nWritten)

	return g.writeHeader()
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
	if err := os.Remove(g.filename); err != nil {
		return err
	}
	return os.Remove(g.filename + HeaderFileSuffix)
}

////////////////////////////////////////////////////////////////////////////////

func (g *GPFile) open(flags int) (err error) {
	if g.file != nil {
		return fmt.Errorf("File %s is already open", g.filename)
	}

	// Open file for append, create if not exists
	g.file, err = os.OpenFile(g.filename, flags, defaultPermissions)
	g.fileBuffer = bufio.NewWriter(g.file)

	return
}

func (g *GPFile) readHeader() error {

	// Check if a header file exists for this file and open the file for buffered
	// reading
	gpfHeaderFile := g.filename + HeaderFileSuffix
	gpfHeader, err := os.OpenFile(gpfHeaderFile, os.O_RDONLY, defaultPermissions)
	if err == nil {

		g.header = storage.BlockHeader{
			Blocks: make(map[int64]storage.Block),
		}
		buffer := bufio.NewReader(gpfHeader)
		scanner := bufio.NewScanner(buffer)

		// Read the global header information and all individual blocks
		var (
			ts          int64
			curOffset   int
			block       storage.Block
			encoderType encoders.Type
		)
		scanner.Scan()
		_, err := fmt.Sscanf(scanner.Text(), "v%d,%d,%d", &g.header.Version, &g.header.CurrentOffset, &encoderType)
		if err != nil {
			return err
		}
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Count(line, ",") == 2 {
				if _, err := fmt.Sscanf(scanner.Text(), "%d,%d,%d", &ts, &block.Len, &block.RawLen); err != nil {
					return err
				}
				block.EncoderType = encoderType
			} else {
				if _, err := fmt.Sscanf(scanner.Text(), "%d,%d,%d,%d", &ts, &block.Len, &block.RawLen, &block.EncoderType); err != nil {
					return err
				}
			}

			block.Offset = int64(curOffset)
			curOffset += block.Len
			g.header.Blocks[ts] = block
		}

		return scanner.Err()
	}

	// If the file doesn't exist, do nothing, otherwise throw the encountered error
	if !os.IsNotExist(err) {
		return err
	}

	// If the file has been opened in read mode, the header file MUST exist, otherwise
	// the file is invalid (e.g. from a legacy DB format)
	if g.accessMode == ModeRead {
		return fmt.Errorf("GPFile invalid: Missing header file %s", gpfHeaderFile)
	}

	// Initialize a new header
	g.header = storage.BlockHeader{
		Blocks:  make(map[int64]storage.Block),
		Version: headerVersion,
	}

	return nil
}

func (g *GPFile) writeHeader() error {

	// Open the header file for buffered writing
	gpfHeaderFile := g.filename + HeaderFileSuffix
	gpfHeader, err := os.OpenFile(gpfHeaderFile, os.O_CREATE|os.O_WRONLY, defaultPermissions)
	if err != nil {
		return err
	}
	defer gpfHeader.Close()
	buffer := bufio.NewWriter(gpfHeader)

	// Write the global header information and all individual blocks
	var curOffset int
	if _, err := fmt.Fprintf(buffer, "v%d,%d,%d\n", g.header.Version, g.header.CurrentOffset, g.defaultEncoderType); err != nil {
		return err
	}
	for _, block := range g.header.OrderedList() {
		if block.EncoderType != g.defaultEncoderType {
			if _, err := fmt.Fprintf(buffer, "%d,%d,%d,%d\n", block.Timestamp, block.Len, block.RawLen, block.EncoderType); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(buffer, "%d,%d,%d\n", block.Timestamp, block.Len, block.RawLen); err != nil {
				return err
			}
		}
		curOffset += block.Len
	}

	// Flush the buffer
	return buffer.Flush()
}
