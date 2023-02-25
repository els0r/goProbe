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

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage"
	"github.com/els0r/goProbe/pkg/types"
)

const (

	// EpochDay is one day in seconds
	EpochDay int64 = 86400

	metadataFileName = ".blockmeta"
)

// Global memory pool used to minimize allocations
var metaDataMemPool = NewMemPoolNoLimit()

// TrafficMetadata denotes a serializable set of metadata information about traffic stats
type TrafficMetadata struct {
	NumV4Entries uint64
	NumV6Entries uint64
	NumDrops     int
}

type Stats struct {
	Counts  types.Counters
	Traffic TrafficMetadata
}

// NumFlows returns the total number of flows
func (t TrafficMetadata) NumFlows() uint64 {
	return t.NumV4Entries + t.NumV6Entries
}

// Add computes the sum of two sets of TrafficMetadata
func (t TrafficMetadata) Add(t2 TrafficMetadata) TrafficMetadata {
	t.NumDrops += t2.NumDrops
	t.NumV4Entries += t2.NumV4Entries
	t.NumV6Entries += t2.NumV6Entries
	return t
}

// Sub computes the difference of two sets of TrafficMetadata
func (t TrafficMetadata) Sub(t2 TrafficMetadata) TrafficMetadata {
	t.NumDrops -= t2.NumDrops
	t.NumV4Entries -= t2.NumV4Entries
	t.NumV6Entries -= t2.NumV6Entries
	return t
}

// Metadata denotes a serializable set of metadata (both globally and per-block)
type Metadata struct {
	BlockMetadata [types.ColIdxCount]*storage.BlockHeader
	BlockTraffic  []TrafficMetadata

	Stats
	Version int
}

// newMetadata initializes a new Metadata set (internal / serialization use only)
func newMetadata() *Metadata {
	m := Metadata{
		BlockTraffic: make([]TrafficMetadata, 0),
		Version:      headerVersion,
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
	dirPath    string   // GPDir path (up to GPDir timestanp)
	metaPath   string   // Full path to GPDir metadata
	timestamp  int64    // Timestamp of GPDir
	accessMode int      // Access mode (also forwarded to all GPFiles)

	*Metadata
}

// NewDir instantiates a new directory (doesn't yet do anything)
func NewDir(basePath string, timestamp int64, accessMode int, options ...Option) *GPDir {
	obj := GPDir{
		basePath: strings.TrimSuffix(basePath, "/"),

		timestamp:  DirTimestamp(timestamp),
		accessMode: accessMode,
		options:    options,
	}
	obj.dirPath = filepath.Join(basePath, strconv.Itoa(int(obj.timestamp)))
	obj.metaPath = filepath.Join(obj.dirPath, metadataFileName)
	return &obj
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
	return d.BlockTraffic[blockIdx].NumV4Entries
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
func (d *GPDir) WriteBlocks(timestamp int64, blockTraffic TrafficMetadata, counters types.Counters, dbData [types.ColIdxCount][]byte) error {
	for colIdx := types.ColumnIndex(0); colIdx < types.ColIdxCount; colIdx++ {

		// Load column if required
		_, err := d.Column(colIdx)
		if err != nil {
			return err
		}

		// Write data to column file
		if err := d.gpFiles[colIdx].writeBlock(timestamp, dbData[colIdx]); err != nil {
			return err
		}
	}

	// Update global block info / counters
	d.Metadata.BlockTraffic = append(d.Metadata.BlockTraffic, blockTraffic)
	d.Metadata.Traffic = d.Metadata.Traffic.Add(blockTraffic)
	d.Metadata.Counts = d.Metadata.Counts.Add(counters)

	return nil
}

// TimeRange returns the first and last timestamp covered by this GPDir
func (d *GPDir) TimeRange() (first int64, last int64) {
	return d.BlockMetadata[0].Blocks()[0].Timestamp,
		d.BlockMetadata[0].Blocks()[d.BlockMetadata[0].NBlocks()-1].Timestamp
}

// Unmarshal reads and unmarshals a serialized metadata set into the GPDir instance
func (d *GPDir) Unmarshal(r ReadWriteSeekCloser) error {

	// Read the file into a buffer to avoid any allocation and maximize throughput
	memFile, err := NewMemFile(r, metaDataMemPool)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := memFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	data := memFile.Data()
	if len(data) < 16 {
		return fmt.Errorf("input data too small to be a GPDir metadata header (len: %d)", len(data))
	}

	d.Metadata = newMetadata()

	d.Metadata.Version = int(binary.BigEndian.Uint64(data[0:8]))            // Get header version
	nBlocks := int(binary.BigEndian.Uint64(data[8:16]))                     // Get flat nummber of blocks
	d.Metadata.Traffic.NumV4Entries = binary.BigEndian.Uint64(data[16:24])  // Get global number of IPv4 flows
	d.Metadata.Traffic.NumV6Entries = binary.BigEndian.Uint64(data[24:32])  // Get global number of IPv6 flows
	d.Metadata.Traffic.NumDrops = int(binary.BigEndian.Uint64(data[32:40])) // Get global number of dropped packets
	d.Metadata.Counts.BytesRcvd = binary.BigEndian.Uint64(data[40:48])      // Get global Counters (BytesRcvd)
	d.Metadata.Counts.BytesSent = binary.BigEndian.Uint64(data[48:56])      // Get global Counters (BytesSent)
	d.Metadata.Counts.PacketsRcvd = binary.BigEndian.Uint64(data[56:64])    // Get global Counters (PacketsRcvd)
	d.Metadata.Counts.PacketsSent = binary.BigEndian.Uint64(data[64:72])    // Get global Counters (PacketsSent)
	pos := 72

	// Get block information
	for i := 0; i < int(types.ColIdxCount); i++ {
		d.BlockMetadata[i].CurrentOffset = int64(binary.BigEndian.Uint64(data[pos : pos+8]))
		d.BlockMetadata[i].BlockList = make([]storage.BlockAtTime, nBlocks)
		pos += 8
		curOffset := 0
		for j := 0; j < nBlocks; j++ {
			d.BlockMetadata[i].BlockList[j].Offset = int64(curOffset)
			d.BlockMetadata[i].BlockList[j].Len = int(binary.BigEndian.Uint32(data[pos : pos+4]))
			d.BlockMetadata[i].BlockList[j].RawLen = int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
			d.BlockMetadata[i].BlockList[j].EncoderType = encoders.Type(data[pos+8])
			pos += 9

			curOffset += d.BlockMetadata[i].BlockList[j].Len
		}
	}

	// Get Metadata.NumIPV4Entries
	d.BlockTraffic = make([]TrafficMetadata, nBlocks)
	lastTimestamp := int64(binary.BigEndian.Uint64(data[pos : pos+8]))
	pos += 8
	for i := 0; i < nBlocks; i++ {
		d.BlockTraffic[i].NumV4Entries = uint64(binary.BigEndian.Uint32(data[pos : pos+4]))
		d.BlockTraffic[i].NumV6Entries = uint64(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		d.BlockTraffic[i].NumDrops = int(binary.BigEndian.Uint32(data[pos+8 : pos+12]))
		thisTimestamp := lastTimestamp + int64(binary.BigEndian.Uint32(data[pos+12:pos+16]))
		for j := 0; j < int(types.ColIdxCount); j++ {
			d.BlockMetadata[j].BlockList[i].Timestamp = thisTimestamp
		}
		lastTimestamp = thisTimestamp
		pos += 16
	}

	return nil
}

// Marshal marshals and writes the metadata of the GPDir instance into serialized metadata set
func (d *GPDir) Marshal(w ReadWriteSeekCloser) error {

	nBlocks := len(d.BlockTraffic)
	size := 8 + // Overall number of blocks
		8 + // Metadata.Version
		8 + // Metadata.NumV4Entries
		8 + // Metadata.NumV6Entries
		8 + // Metadata.NumDrops
		8*4 + // Metadata.Counts
		8 + // Metadata.BlockMetadata (first timestampm)
		nBlocks*4 + // Metadata.GlobalBlockMetadata.NumV4Entries
		nBlocks*4 + // Metadata.GlobalBlockMetadata.NumV6Entries
		nBlocks*4 + // Metadata.GlobalBlockMetadata.NumDrops
		nBlocks*4 + // Metadata.BlockMetadata.BlockList.Timestamp (Delta)
		int(types.ColIdxCount)*8 + // Metadata.BlockMetadata.CurrentOffset
		nBlocks*int(types.ColIdxCount)*4 + // Metadata.BlockMetadata.BlockList.Len
		nBlocks*int(types.ColIdxCount)*4 + // Metadata.BlockMetadata.BlockList.RawLen
		nBlocks*int(types.ColIdxCount) // Metadata.BlockMetadata.BlockList.Block.EncoderType

	// Note: Lengths and timestamp deltas are encoded as uint32s, allowing for a maximum block (!) size of
	// 4 GiB (uncompressed / compressed).
	// If a single block is larger than that (or the time between consecutive block writes) is larger than that,
	// something is _very_ wrong

	// TODO: Add safety / bounds-check

	// Fetch a buffer from the pool
	data := metaDataMemPool.Get(size)
	defer metaDataMemPool.Put(data)

	binary.BigEndian.PutUint64(data[0:8], uint64(d.Metadata.Version))                // Store header version
	binary.BigEndian.PutUint64(data[8:16], uint64(nBlocks))                          // Store flat nummber of blocks
	binary.BigEndian.PutUint64(data[16:24], uint64(d.Metadata.Traffic.NumV4Entries)) // Store global number of IPv4 flows
	binary.BigEndian.PutUint64(data[24:32], uint64(d.Metadata.Traffic.NumV6Entries)) // Store global number of IPv6 flows
	binary.BigEndian.PutUint64(data[32:40], uint64(d.Metadata.Traffic.NumDrops))     // Store global number of dropped packets
	binary.BigEndian.PutUint64(data[40:48], uint64(d.Metadata.Counts.BytesRcvd))     // Store global Counters (BytesRcvd)
	binary.BigEndian.PutUint64(data[48:56], uint64(d.Metadata.Counts.BytesSent))     // Store global Counters (BytesSent)
	binary.BigEndian.PutUint64(data[56:64], uint64(d.Metadata.Counts.PacketsRcvd))   // Store global Counters (PacketsRcvd)
	binary.BigEndian.PutUint64(data[64:72], uint64(d.Metadata.Counts.PacketsSent))   // Store global Counters (PacketsSent)
	pos := 72

	if nBlocks > 0 {

		// Store block information
		for i := 0; i < int(types.ColIdxCount); i++ {
			binary.BigEndian.PutUint64(data[pos:pos+8], uint64(d.BlockMetadata[i].CurrentOffset))
			pos += 8
			for _, block := range d.BlockMetadata[i].BlockList {
				binary.BigEndian.PutUint32(data[pos:pos+4], uint32(block.Len))
				binary.BigEndian.PutUint32(data[pos+4:pos+8], uint32(block.RawLen))
				data[pos+8] = byte(block.EncoderType)
				pos += 9
			}
		}

		// Store Metadata.NumIPV4Entries
		lastTimestamp := d.BlockMetadata[0].BlockList[0].Timestamp
		binary.BigEndian.PutUint64(data[pos:pos+8], uint64(lastTimestamp))
		pos += 8
		for i := 0; i < len(d.BlockTraffic); i++ {
			binary.BigEndian.PutUint32(data[pos:pos+4], uint32(d.BlockTraffic[i].NumV4Entries))
			binary.BigEndian.PutUint32(data[pos+4:pos+8], uint32(d.BlockTraffic[i].NumV6Entries))
			binary.BigEndian.PutUint32(data[pos+8:pos+12], uint32(d.BlockTraffic[i].NumDrops))
			binary.BigEndian.PutUint32(data[pos+12:pos+16], uint32(d.BlockMetadata[0].BlockList[i].Timestamp-lastTimestamp))
			lastTimestamp = d.BlockMetadata[0].BlockList[i].Timestamp
			pos += 16
		}
	}

	n, err := w.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) || n != size {
		return fmt.Errorf("invalid number of bytes written, want %d, have %d", len(data), n)
	}
	data = nil

	return nil
}

// Path returns the path of the GPDir (up to the timestamp)
func (d *GPDir) Path() string {
	return d.dirPath
}

// MetadataPath returns the full path of the GPDir metadata file
func (d *GPDir) MetadataPath() string {
	return d.metaPath
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
		d.Metadata.BlockTraffic = nil
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
		if d.gpFiles[colIdx], err = New(filepath.Join(d.Path(), types.ColumnFileNames[colIdx]+FileSuffix), d.BlockMetadata[colIdx], d.accessMode, d.options...); err != nil {
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
