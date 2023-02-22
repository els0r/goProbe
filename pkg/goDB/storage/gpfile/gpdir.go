package gpfile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/edsrzf/mmap-go"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage"
	"github.com/els0r/goProbe/pkg/types"
)

const (

	// EpochDay is one day in seconds
	EpochDay int64 = 86400

	metadataFileName = ".blockmeta"
)

// var metaDataPool = NewMemPool(64)

// Metadata denotes a serializable set of metadata (both globally and per-block)
type Metadata struct {
	BlockMetadata     [types.ColIdxCount]*storage.BlockHeader
	BlockNumV4Entries []uint64
	Version           int
}

// newMetadata initializes a new Metadata set (internal / serialization use only)
func newMetadata() *Metadata {
	m := Metadata{
		BlockNumV4Entries: make([]uint64, 0),
		Version:           headerVersion,
	}
	for i := 0; i < int(types.ColIdxCount); i++ {
		m.BlockMetadata[i] = &storage.BlockHeader{
			BlockList: make([]storage.BlockAtTime, 0),
		}
	}
	return &m
}

// GPDir denotes a timestamped goDB directory (usually a daily set of blocks)
type GPDir struct {
	gpFiles [types.ColIdxCount]*GPFile // Set of GPFile (lazy-load)

	options    []Option // Options (forwarded to all GPFiles)
	basePath   string   // goDB base path (up to interface)
	timestamp  int64    // Timestamp of GPDir
	accessMode int      // Access mode (also forwarded to all GPFiles)

	*Metadata
}

// NewDir instantiates a new directory (doesn't yet do anything)
func NewDir(basePath string, timestamp int64, accessMode int, options ...Option) *GPDir {
	return &GPDir{
		basePath:   strings.TrimSuffix(basePath, "/"),
		timestamp:  DirTimestamp(timestamp),
		accessMode: accessMode,
		options:    options,
	}
}

// Open accesses the metadata and prepares the GPDir for reading / writing
func (d *GPDir) Open() error {

	// If the directory has been opened in write mode, ensure it is created if required
	if d.accessMode == ModeWrite {
		if err := d.createIfRequired(); err != nil {
			return err
		}
	}

	// Attempt to read the metadata from file
	metadataFile, err := os.Open(d.MetadataPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {

			// In read mode the metadata file has to be present, otherwise we instantiate
			// an empty one
			if d.accessMode == ModeRead {
				return fmt.Errorf("metadata file `%s` missing", d.MetadataPath())
			} else {
				d.Metadata = newMetadata()
			}
		} else {
			return fmt.Errorf("error reading metadata file `%s`: %w", d.MetadataPath(), err)
		}
	} else {

		// Deserialize and close underlying file after reading is complete
		defer func() {
			if cerr := metadataFile.Close(); cerr != nil && err != nil {
				err = cerr
			}
		}()
		if err := d.Unmarshal(metadataFile); err != nil {
			return fmt.Errorf("error decoding metadata file `%s`: %w", d.MetadataPath(), err)
		}
	}

	return nil
}

// NumIPv4EntriesAtIndex returns the number of IPv4 entries for a given block index
func (d *GPDir) NumIPv4EntriesAtIndex(blockIdx int) uint64 {
	return d.BlockNumV4Entries[blockIdx]
}

// ReadBlockAtIndex returns the block for a specified block index from the underlying GPFile
func (d *GPDir) ReadBlockAtIndex(colIdx types.ColumnIndex, blockIdx int) ([]byte, error) {

	// Load column if required
	_, err := d.Column(colIdx)
	if err != nil {
		return nil, err
	}

	// Read block data from file
	return d.gpFiles[colIdx].ReadBlockAtIndex(blockIdx)
}

// WriteBlocks writes a set of blocks to the underlying GPFiles and updates the metadata
func (d *GPDir) WriteBlocks(timestamp int64, numV4Entries uint64, dbData [types.ColIdxCount][]byte) error {
	for colIdx := types.ColumnIndex(0); colIdx < types.ColIdxCount; colIdx++ {

		// Load column if required
		_, err := d.Column(colIdx)
		if err != nil {
			return err
		}

		// Write data to column file
		if err := d.gpFiles[colIdx].WriteBlock(timestamp, dbData[colIdx]); err != nil {
			return err
		}
	}

	// Update IPv4 entry counter
	d.Metadata.BlockNumV4Entries = append(d.Metadata.BlockNumV4Entries, numV4Entries)

	return nil
}

// TimeRange returns the first and last timestamp covered by this GPDir
func (d *GPDir) TimeRange() (first int64, last int64) {
	return d.BlockMetadata[0].Blocks()[0].Timestamp,
		d.BlockMetadata[0].Blocks()[d.BlockMetadata[0].NBlocks()-1].Timestamp
}

// Unmarshal reads and unmarshals a serialized metadata set into the GPDir instance
func (d *GPDir) Unmarshal(r *os.File) error {

	// Memor-map the file for reading to avoid any allocation and maximize throughput
	data, err := mmap.Map(r, mmap.RDONLY, 0)
	if err != nil {
		return err
	}
	defer func() {
		if uerr := data.Unmap(); uerr != nil && err == nil {
			err = uerr
		}
	}()

	d.Metadata = newMetadata()

	// Get flat nummber of blocks
	nBlocks := int(binary.BigEndian.Uint64(data[0:8]))

	// Get header version
	d.Metadata.Version = int(binary.BigEndian.Uint64(data[8:16]))
	pos := 16

	// Get block information
	for i := 0; i < int(types.ColIdxCount); i++ {
		d.BlockMetadata[i].CurrentOffset = int64(binary.BigEndian.Uint64(data[pos : pos+8]))
		d.BlockMetadata[i].BlockList = make([]storage.BlockAtTime, nBlocks)
		pos += 8
		for j := 0; j < nBlocks; j++ {
			d.BlockMetadata[i].BlockList[j].EncoderType = encoders.Type(data[pos])
			d.BlockMetadata[i].BlockList[j].Offset = int64(binary.BigEndian.Uint64(data[pos+1 : pos+9]))
			d.BlockMetadata[i].BlockList[j].Len = int(binary.BigEndian.Uint64(data[pos+9 : pos+17]))
			d.BlockMetadata[i].BlockList[j].RawLen = int(binary.BigEndian.Uint64(data[pos+17 : pos+25]))
			d.BlockMetadata[i].BlockList[j].Timestamp = int64(binary.BigEndian.Uint64(data[pos+25 : pos+33]))
			pos += 33
		}
	}

	// Get Metadata.NumIPV4Entries
	d.BlockNumV4Entries = make([]uint64, nBlocks)
	for i := 0; i < nBlocks; i++ {
		d.BlockNumV4Entries[i] = binary.BigEndian.Uint64(data[pos : pos+8])
		pos += 8
	}

	return nil
}

// Marshal marshals and writes the metadata of the GPDir instance into serialized metadata set
func (d *GPDir) Marshal(w *os.File) error {

	nBlocks := len(d.BlockNumV4Entries)
	size := 8 + // Overall number of blocks
		8 + // Metadata.Version
		nBlocks*8 + // Metadata.NumIPV4Entries (8 bytes each)
		int(types.ColIdxCount)*8 + // Metadata.BlockMetadata.CurrentOffset
		nBlocks*int(types.ColIdxCount) + // Metadata.BlockMetadata.BlockList.Block.EncoderType
		nBlocks*int(types.ColIdxCount)*8 + // Metadata.BlockMetadata.BlockList.Timestamp
		nBlocks*int(types.ColIdxCount)*8 + // Metadata.BlockMetadata.BlockList.Offset
		nBlocks*int(types.ColIdxCount)*8 + // Metadata.BlockMetadata.BlockList.Len
		nBlocks*int(types.ColIdxCount)*8 // Metadata.BlockMetadata.BlockList.RawLen

	data := make([]byte, size)
	// metaDataPool.Get(size)
	// defer metaDataPool.Put(data)

	// Store flat nummber of blocks
	binary.BigEndian.PutUint64(data[0:8], uint64(nBlocks))

	// Store header version
	binary.BigEndian.PutUint64(data[8:16], uint64(d.Metadata.Version))
	pos := 16

	// Store block information
	for i := 0; i < int(types.ColIdxCount); i++ {
		binary.BigEndian.PutUint64(data[pos:pos+8], uint64(d.BlockMetadata[i].CurrentOffset))
		pos += 8
		for _, block := range d.BlockMetadata[i].BlockList {
			data[pos] = byte(block.EncoderType)
			binary.BigEndian.PutUint64(data[pos+1:pos+9], uint64(block.Offset))
			binary.BigEndian.PutUint64(data[pos+9:pos+17], uint64(block.Len))
			binary.BigEndian.PutUint64(data[pos+17:pos+25], uint64(block.RawLen))
			binary.BigEndian.PutUint64(data[pos+25:pos+33], uint64(block.Timestamp))
			pos += 33
		}
	}

	// Store Metadata.NumIPV4Entries
	for i := 0; i < len(d.BlockNumV4Entries); i++ {
		binary.BigEndian.PutUint64(data[pos:pos+8], uint64(d.BlockNumV4Entries[i]))
		pos += 8
	}
	// if err := data.Flush(); err != nil {
	// 	return err
	// }
	// if err := data.Unmap(); err != nil {
	// 	return err
	// }

	n, err := w.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("invalid number of bytes written, want %d, have %d", len(data), n)
	}
	data = nil

	return nil
}

// Path returns the path of the GPDir (up to the timestamp)
func (d *GPDir) Path() string {
	return fmt.Sprintf("%s/%d", d.basePath, d.timestamp)
}

// MetadataPath returns the full path of the GPDir metadata file
func (d *GPDir) MetadataPath() string {
	return fmt.Sprintf("%s/%d/%s", d.basePath, d.timestamp, metadataFileName)
}

// NBlocks returns the number of blocks in this GPDir
func (d *GPDir) NBlocks() int {
	return d.BlockMetadata[0].NBlocks()
}

// Close closes all underlying open GPFiles and cleans up resources
func (d *GPDir) Close() error {

	// Close all open GPFiles
	var errs []error
	for i := 0; i < int(types.ColIdxCount); i++ {
		if d.gpFiles[i] != nil {
			if err := d.gpFiles[i].Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Ensure resources are marked for cleanup
	defer func() {
		d.Metadata.BlockNumV4Entries = nil
		for i := 0; i < int(types.ColIdxCount); i++ {
			d.Metadata.BlockMetadata[i].BlockList = nil
			d.Metadata.BlockMetadata[i] = nil
		}
	}()

	// In write mode, update the metadata on disk (creating / overwriting)
	if d.accessMode == ModeWrite {
		metadataFile, err := os.OpenFile(d.MetadataPath(), os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := metadataFile.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}()

		return d.Marshal(metadataFile)
	}

	return nil
}

// column returns the underlying GPFile for a specified column (lazy-access)
func (d *GPDir) Column(colIdx types.ColumnIndex) (*GPFile, error) {
	if d.gpFiles[colIdx] == nil {
		var err error
		if d.gpFiles[colIdx], err = New(filepath.Join(d.Path(), fmt.Sprintf("%s%s", types.ColumnFileNames[colIdx], FileSuffix)), d.BlockMetadata[colIdx], d.accessMode, d.options...); err != nil {
			return nil, err
		}
	}

	return d.gpFiles[colIdx], nil
}

// createIfRequired created the underlying path structure (if missing)
func (d *GPDir) createIfRequired() error {
	path := filepath.Join(d.basePath, strconv.FormatInt(d.timestamp, 10))
	return os.MkdirAll(path, 0755)
}

// DirTimestamp returns timestamp rounded down to the nearest directory time frame (usually a day)
func DirTimestamp(timestamp int64) int64 {
	return (timestamp / EpochDay) * EpochDay
}
