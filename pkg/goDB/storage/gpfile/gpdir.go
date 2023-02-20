package gpfile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage"
	"github.com/els0r/goProbe/pkg/types"
)

const (

	// EpochDay is one day in seconds
	EpochDay int64 = 86400

	metadataFileName = "blockmeta.json"
)

var metaDataPool = NewMemPool()

type intPair struct {
	timestamp int64
	num       uint64
}

type Metadata struct {
	BlockMetadata     [types.ColIdxCount]*storage.BlockHeader
	BlockNumV4Entries map[int64]uint64
	Version           int
}

func newMetadata() *Metadata {
	m := Metadata{
		BlockNumV4Entries: make(map[int64]uint64),
		Version:           headerVersion,
	}
	for i := 0; i < int(types.ColIdxCount); i++ {
		m.BlockMetadata[i] = &storage.BlockHeader{
			Blocks:    make(map[int64]storage.Block),
			BlockList: make([]storage.BlockAtTime, 0),
		}
	}
	return &m
}

type GPDir struct {
	gpFiles [types.ColIdxCount]*GPFile

	options    []Option
	basePath   string
	timestamp  int64
	accessMode int

	*Metadata
}

func NewDir(basePath string, timestamp int64, accessMode int, options ...Option) (*GPDir, error) {

	dir := GPDir{
		basePath:   strings.TrimSuffix(basePath, "/"),
		timestamp:  DirTimestamp(timestamp),
		accessMode: accessMode,
		options:    options,
	}

	// If the directory has been opened in write mode, ensure it exists
	if dir.accessMode == ModeWrite {
		if err := dir.createIfRequired(); err != nil {
			return nil, err
		}
	}

	// Attempt to read the metadata from file
	metadataFile, err := os.Open(dir.MetadataPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if dir.accessMode == ModeRead {
				return nil, fmt.Errorf("metadata file `%s` missing", dir.MetadataPath())
			} else {
				dir.Metadata = newMetadata()
			}
		} else {
			return nil, fmt.Errorf("error reading metadata file `%s`: %w", dir.MetadataPath(), err)
		}
	} else {
		defer metadataFile.Close()
		if err := dir.Deserialize(metadataFile); err != nil {
			return nil, fmt.Errorf("error decoding metadata file `%s`: %w", dir.MetadataPath(), err)
		}
	}

	return &dir, nil
}

func (d *GPDir) GetNumIPv4Entries(timestamp int64) uint64 {
	return d.BlockNumV4Entries[timestamp]
}

func (d *GPDir) Column(colIdx types.ColumnIndex) (*GPFile, error) {
	if d.gpFiles[colIdx] == nil {
		var err error
		if d.gpFiles[colIdx], err = New(filepath.Join(d.Path(), fmt.Sprintf("%s%s", types.ColumnFileNames[colIdx], FileSuffix)), d.BlockMetadata[colIdx], d.accessMode, d.options...); err != nil {
			return nil, err
		}
	}

	return d.gpFiles[colIdx], nil
}

func (d *GPDir) ReadBlock(colIdx types.ColumnIndex, timestamp int64) ([]byte, error) {

	// Load column if required
	_, err := d.Column(colIdx)
	if err != nil {
		return nil, err
	}

	// Read block data from file
	return d.gpFiles[colIdx].ReadBlock(timestamp)
}

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
	d.Metadata.BlockNumV4Entries[timestamp] = numV4Entries

	return nil
}

// TODO: Customize encoding (deduplicate map / slice info)
func (d *GPDir) Deserialize(r ReadWriteSeekCloser) error {

	memFile, err := NewMemFile(r, metaDataPool)
	if err != nil {
		return fmt.Errorf("failed to read data for deserialization: %w", err)
	}
	defer memFile.Close()

	data := memFile.Data()
	d.Metadata = newMetadata()

	// Get flat nummber of blocks
	nBlocks := int(binary.BigEndian.Uint64(data[0:8]))

	// Get header version
	d.Metadata.Version = int(binary.BigEndian.Uint64(data[8:16]))
	pos := 16

	// Get block information
	var block storage.BlockAtTime
	for i := 0; i < int(types.ColIdxCount); i++ {
		d.BlockMetadata[i].CurrentOffset = int64(binary.BigEndian.Uint64(data[pos : pos+8]))
		pos += 8
		for j := 0; j < nBlocks; j++ {
			block.EncoderType = encoders.Type(data[pos])
			block.Offset = int64(binary.BigEndian.Uint64(data[pos+1 : pos+9]))
			block.Len = int(binary.BigEndian.Uint64(data[pos+9 : pos+17]))
			block.RawLen = int(binary.BigEndian.Uint64(data[pos+17 : pos+25]))
			block.Timestamp = int64(binary.BigEndian.Uint64(data[pos+25 : pos+33]))
			d.BlockMetadata[i].BlockList = append(d.BlockMetadata[i].BlockList, block)
			d.BlockMetadata[i].Blocks[block.Timestamp] = block.Block
			pos += 33
		}
	}

	// Get Metadata.NumIPV4Entries
	for i := 0; i < nBlocks; i++ {
		d.BlockNumV4Entries[d.BlockMetadata[0].BlockList[i].Timestamp] = binary.BigEndian.Uint64(data[pos : pos+8])
		pos += 8
	}

	return nil
}

// TODO: Customize encoding (deduplicate map / slice info)
func (d *GPDir) Serialize(w ReadWriteSeekCloser) error {

	nBlocks := len(d.BlockNumV4Entries)
	size := 8 + // Overall number of blocks
		int(types.ColIdxCount)*8 + // Metadata.BlockMetadata.Version
		nBlocks*8 + // Metadata.NumIPV4Entries (8 bytes each)
		int(types.ColIdxCount)*8 + // Metadata.BlockMetadata.CurrentOffset
		nBlocks*int(types.ColIdxCount)*8 + // Metadata.BlockMetadata.BlockList.Timestamp
		nBlocks*int(types.ColIdxCount) + // Metadata.BlockMetadata.BlockList.Block.EncoderType
		nBlocks*int(types.ColIdxCount)*8 + // Metadata.BlockMetadata.BlockList.Offset
		nBlocks*int(types.ColIdxCount)*8 + // Metadata.BlockMetadata.BlockList.Len
		nBlocks*int(types.ColIdxCount)*8 // Metadata.BlockMetadata.BlockList.RawLen

	data := metaDataPool.Get()
	if cap(data) < size {
		data = make([]byte, size)
	}
	defer metaDataPool.Put(data)

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
	pairs := make([]intPair, 0, nBlocks)
	for k, v := range d.BlockNumV4Entries {
		pairs = append(pairs, intPair{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].timestamp < pairs[j].timestamp
	})
	for i := 0; i < len(d.BlockNumV4Entries); i++ {
		binary.BigEndian.PutUint64(data[pos:pos+8], uint64(pairs[i].num))
		pos += 8
	}

	n, err := w.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("invalid number of bytes written, want %d, have %d", len(data), n)
	}

	return nil
}

func (d *GPDir) Path() string {
	return fmt.Sprintf("%s/%d", d.basePath, d.timestamp)
}

func (d *GPDir) MetadataPath() string {
	return fmt.Sprintf("%s/%d/%s", d.basePath, d.timestamp, metadataFileName)
}

func (d *GPDir) Blocks() []storage.BlockAtTime {
	return d.BlockMetadata[0].OrderedList()
}

func (d *GPDir) Close() error {
	for i := 0; i < int(types.ColIdxCount); i++ {
		if d.gpFiles[i] != nil {
			if err := d.gpFiles[i].Close(); err != nil {
				// TODO: Accumulate errors and try to close all, then return
				return err
			}
		}
	}

	if d.accessMode == ModeWrite {
		metadataFile, err := os.OpenFile(d.MetadataPath(), os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer metadataFile.Close()

		return d.Serialize(metadataFile)
	}

	return nil
}

func (d *GPDir) createIfRequired() error {
	path := filepath.Join(d.basePath, strconv.FormatInt(d.timestamp, 10))
	return os.MkdirAll(path, 0755)
}

// DirTimestamp returns timestamp rounded down to the nearest directory time frame (usually a day)
func DirTimestamp(timestamp int64) int64 {
	return (timestamp / EpochDay) * EpochDay
}
