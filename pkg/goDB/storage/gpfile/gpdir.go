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
	"time"

	"github.com/els0r/goProbe/v4/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/v4/pkg/goDB/storage"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/fako1024/gotools/concurrency"
)

const (

	// EpochDay is one day in seconds
	EpochDay int64 = 86400

	maxUint32 = 1<<32 - 1 // 4294967295
)

var (

	// Global memory pool used to minimize allocations
	metaDataMemPool = concurrency.NewMemPoolNoLimit()

	// ErrExceedsEncodingSize covers edge case scenarios where a block might (theoretically)
	// contain data that exceeds the encoding width of 32-bit
	ErrExceedsEncodingSize = errors.New("data size exceeds maximum bit width for encoding")

	// ErrInputSizeTooSmall is thrown if the input size for desiralization is too small to be valid
	ErrInputSizeTooSmall = errors.New("input size too small to be a GPDir metadata header")

	// ErrDirNotOpen denotes that a GPDir is not (yet) open or has been closed
	ErrDirNotOpen = errors.New("GPDir not open, call Open() first")

	// ErrInvalidDirName denotes that the provided name for the GPDir is invalid
	ErrInvalidDirName = errors.New("invalid GPDir path / name")
)

// GPDir denotes a timestamped goDB directory (usually a daily set of blocks)
type GPDir struct {
	gpFiles [types.ColIdxCount]*GPFile // Set of GPFile (lazy-load)

	options          []Option    // Options (forwarded to all GPFiles)
	basePath         string      // goDB base path (up to interface)
	dirMonthPath     string      // GPDir path (up to GPDir month)
	dirTimestampPath string      // GPDir path (up to GPDir timestanp)
	dirPath          string      // GPDir path (full path including GPDir timestanp and potential metadata suffix)
	metaPath         string      // Full path to GPDir metadata
	accessMode       int         // Access mode (also forwarded to all GPFiles)
	permissions      os.FileMode // Permissions (also forwarded to all GPFiles)

	isOpen bool
	*Metadata
}

// ExtractTimestampMetadataSuffix is a convenience function that performs timestamp (and potentially
// metadata prefix) extraction for the GPDir path / directory name
func ExtractTimestampMetadataSuffix(filename string) (timestamp int64, metadataSuffix string, err error) {

	// Split by delimeter and perform minumum validation
	splitName := strings.Split(filename, "_")
	if len(splitName) == 0 {
		err = ErrInvalidDirName
		return
	}

	// Check if a suffix is present and extract it
	if len(splitName) > 1 {
		metadataSuffix = splitName[1]
	}

	// Parse timestamp from prefix
	timestamp, err = strconv.ParseInt(splitName[0], 10, 64)

	return
}

// NewDirWriter instantiates a new directory for writing
func NewDirWriter(basePath string, timestamp int64, options ...Option) *GPDir {
	obj := GPDir{
		basePath:    strings.TrimSuffix(basePath, "/"),
		accessMode:  ModeWrite,
		permissions: defaultPermissions,
		options:     options,
	}

	obj.dirTimestampPath, obj.dirPath = genWritePathForTimestamp(basePath, timestamp)
	obj.metaPath = filepath.Join(obj.dirPath, metadataFileName)

	return &obj
}

// NewDirReader instantiates a new directory (doesn't yet do anything except for potentially
// reading / decoding a subset of the metadata from a provided string suffix)
func NewDirReader(basePath string, timestamp int64, metadataSuffix string, options ...Option) *GPDir {
	obj := GPDir{
		basePath:    strings.TrimSuffix(basePath, "/"),
		accessMode:  ModeRead,
		permissions: defaultPermissions,
		options:     options,
	}

	obj.dirMonthPath, obj.dirPath = genReadPathForTimestamp(basePath, timestamp, metadataSuffix)
	obj.metaPath = filepath.Join(obj.dirPath, metadataFileName)

	// If metdadata was provided via a suffix, attempt to read / decode it and fall
	// back to doing nothing in case it fails
	if metadataSuffix != "" {
		obj.setMetadataFromSuffix(metadataSuffix)
	}

	return &obj
}

// Open accesses the metadata and prepares the GPDir for reading / writing
func (d *GPDir) Open(options ...Option) error {

	// append functional options, if any
	d.options = append(d.options, options...)

	// apply functional options
	for _, opt := range d.options {
		opt(d)
	}

	// If the directory has been opened in write mode, ensure it is created if required
	if d.accessMode == ModeWrite {
		if err := d.createIfRequired(); err != nil {
			return err
		}
	}

	// Attempt to read the metadata from file
	metadataFile, err := os.Open(d.MetadataPath())

	switch d.accessMode {
	case ModeWrite:
		// In write mode the metadata file is optional, if it doesn't exist we create a new one
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("error reading metadata file `%s`: %w", d.MetadataPath(), err)
			}
			d.Metadata, d.isOpen = newMetadata(), true
			return nil
		}

	case ModeRead:
		// In read mode the metadata file has to be present, but we attempt to recover
		// it in case it is missing
		if errors.Is(err, fs.ErrNotExist) {
			if err = d.recoverDirPath(); err == nil {
				metadataFile, err = os.Open(d.MetadataPath())
			}
		}
		if err != nil {
			return fmt.Errorf("error reading metadata file `%s`: %w", d.MetadataPath(), err)
		}
	}

	// Deserialize underlying file (closing taking care of in Unmarshal() method)
	if err := d.Unmarshal(metadataFile); err != nil {
		return fmt.Errorf("error decoding metadata file `%s`: %w", d.MetadataPath(), err)
	}

	d.isOpen = true
	return nil
}

// IsOpen returns if the GPFile instance is currently opened
func (d *GPDir) IsOpen() bool {
	return d.isOpen
}

// NumIPv4EntriesAtIndex returns the number of IPv4 entries for a given block index
func (d *GPDir) NumIPv4EntriesAtIndex(blockIdx int) uint64 {
	return d.BlockTraffic[blockIdx].NumV4Entries
}

// NumIPv6EntriesAtIndex returns the number of IPv6 entries for a given block index
func (d *GPDir) NumIPv6EntriesAtIndex(blockIdx int) uint64 {
	return d.BlockTraffic[blockIdx].NumV6Entries
}

// ReadBlockAtIndex returns the block for a specified block index from the underlying GPFile
func (d *GPDir) ReadBlockAtIndex(colIdx types.ColumnIndex, blockIdx int) ([]byte, error) {

	if !d.isOpen {
		return nil, ErrDirNotOpen
	}

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
	d.Metadata.Counts.Add(counters)

	return nil
}

// SetMemPool sets a memory pool (used to access the underlying GPFiles in full-read mode)
func (d *GPDir) SetMemPool(pool concurrency.MemPoolGCable) {
	d.options = append(d.options, WithReadAll(pool))
}

// TimeRange returns the first and last timestamp covered by this GPDir
func (d *GPDir) TimeRange() (first int64, last int64) {
	return d.BlockMetadata[0].Blocks()[0].Timestamp,
		d.BlockMetadata[0].Blocks()[d.BlockMetadata[0].NBlocks()-1].Timestamp
}

const (
	minMetadataFileSize    = 73
	minMetadataFileSizePos = minMetadataFileSize - 1
)

// Unmarshal reads and unmarshals a serialized metadata set into the GPDir instance
func (d *GPDir) Unmarshal(r concurrency.ReadWriteSeekCloser) error {

	// Read the file into a buffer to avoid any allocation and maximize throughput
	memFile, err := concurrency.NewMemFile(r, metaDataMemPool)
	if err != nil {
		return err
	}

	data := memFile.Data()
	if len(data) < minMetadataFileSize {
		return fmt.Errorf("%w (len: %d)", ErrInputSizeTooSmall, len(data))
	}
	_ = data[minMetadataFileSizePos] // Compiler hint

	d.Metadata = newMetadata()

	d.Metadata.Version = binary.BigEndian.Uint64(data[0:8])                // Get header version
	nBlocks := binary.BigEndian.Uint64(data[8:16])                         // Get flat nummber of blocks
	d.Metadata.Traffic.NumV4Entries = binary.BigEndian.Uint64(data[16:24]) // Get global number of IPv4 flows
	d.Metadata.Traffic.NumV6Entries = binary.BigEndian.Uint64(data[24:32]) // Get global number of IPv6 flows
	d.Metadata.Traffic.NumDrops = binary.BigEndian.Uint64(data[32:40])     // Get global number of dropped packets
	d.Metadata.Counts.BytesRcvd = binary.BigEndian.Uint64(data[40:48])     // Get global Counters (BytesRcvd)
	d.Metadata.Counts.BytesSent = binary.BigEndian.Uint64(data[48:56])     // Get global Counters (BytesSent)
	d.Metadata.Counts.PacketsRcvd = binary.BigEndian.Uint64(data[56:64])   // Get global Counters (PacketsRcvd)
	d.Metadata.Counts.PacketsSent = binary.BigEndian.Uint64(data[64:72])   // Get global Counters (PacketsSent)
	pos := minMetadataFileSizePos

	// Get block information
	for i := range int(types.ColIdxCount) {
		d.BlockMetadata[i].CurrentOffset = binary.BigEndian.Uint64(data[pos : pos+8])
		d.BlockMetadata[i].BlockList = make([]storage.BlockAtTime, nBlocks)
		pos += 8
		curOffset := uint64(0)
		for j := range nBlocks {
			d.BlockMetadata[i].BlockList[j].Offset = curOffset
			d.BlockMetadata[i].BlockList[j].Len = binary.BigEndian.Uint32(data[pos : pos+4])
			d.BlockMetadata[i].BlockList[j].RawLen = binary.BigEndian.Uint32(data[pos+4 : pos+8])
			d.BlockMetadata[i].BlockList[j].EncoderType = encoders.Type(data[pos+8])
			pos += 9

			curOffset += uint64(d.BlockMetadata[i].BlockList[j].Len)
		}
	}

	// Get Metadata.NumIPV4Entries
	d.BlockTraffic = make([]TrafficMetadata, nBlocks)
	lastTimestamp := int64(binary.BigEndian.Uint64(data[pos : pos+8]))
	pos += 8
	for i := uint64(0); i < nBlocks; i++ {
		d.BlockTraffic[i].NumV4Entries = uint64(binary.BigEndian.Uint32(data[pos : pos+4]))
		d.BlockTraffic[i].NumV6Entries = uint64(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		d.BlockTraffic[i].NumDrops = uint64(binary.BigEndian.Uint32(data[pos+8 : pos+12]))
		thisTimestamp := lastTimestamp + int64(binary.BigEndian.Uint32(data[pos+12:pos+16]))
		for j := 0; j < int(types.ColIdxCount); j++ {
			d.BlockMetadata[j].BlockList[i].Timestamp = thisTimestamp
		}
		lastTimestamp = thisTimestamp
		pos += 16
	}

	return memFile.Close()
}

// Marshal marshals and writes the metadata of the GPDir instance into serialized metadata set
func (d *GPDir) Marshal(w concurrency.ReadWriteSeekCloser) error {

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

	// Fetch a buffer from the pool
	data := metaDataMemPool.Get(size)
	defer metaDataMemPool.Put(data)

	binary.BigEndian.PutUint64(data[0:8], d.Metadata.Version)                // Store header version
	binary.BigEndian.PutUint64(data[8:16], uint64(nBlocks))                  // Store flat nummber of blocks
	binary.BigEndian.PutUint64(data[16:24], d.Metadata.Traffic.NumV4Entries) // Store global number of IPv4 flows
	binary.BigEndian.PutUint64(data[24:32], d.Metadata.Traffic.NumV6Entries) // Store global number of IPv6 flows
	binary.BigEndian.PutUint64(data[32:40], d.Metadata.Traffic.NumDrops)     // Store global number of dropped packets
	binary.BigEndian.PutUint64(data[40:48], d.Metadata.Counts.BytesRcvd)     // Store global Counters (BytesRcvd)
	binary.BigEndian.PutUint64(data[48:56], d.Metadata.Counts.BytesSent)     // Store global Counters (BytesSent)
	binary.BigEndian.PutUint64(data[56:64], d.Metadata.Counts.PacketsRcvd)   // Store global Counters (PacketsRcvd)
	binary.BigEndian.PutUint64(data[64:72], d.Metadata.Counts.PacketsSent)   // Store global Counters (PacketsSent)
	pos := minMetadataFileSizePos

	if nBlocks > 0 {

		// Store block information
		for i := 0; i < int(types.ColIdxCount); i++ {
			binary.BigEndian.PutUint64(data[pos:pos+8], d.BlockMetadata[i].CurrentOffset)
			pos += 8
			for _, block := range d.BlockMetadata[i].BlockList {

				// Range check
				if block.Len > maxUint32 || block.RawLen > maxUint32 {
					return ErrExceedsEncodingSize
				}

				binary.BigEndian.PutUint32(data[pos:pos+4], block.Len)
				binary.BigEndian.PutUint32(data[pos+4:pos+8], block.RawLen)
				data[pos+8] = byte(block.EncoderType)
				pos += 9
			}
		}

		// Store Metadata.NumIPV4Entries
		lastTimestamp := d.BlockMetadata[0].BlockList[0].Timestamp
		binary.BigEndian.PutUint64(data[pos:pos+8], uint64(lastTimestamp))
		pos += 8
		for i := 0; i < len(d.BlockTraffic); i++ {

			// Range check
			if d.BlockTraffic[i].NumV4Entries > maxUint32 ||
				d.BlockTraffic[i].NumV6Entries > maxUint32 ||
				d.BlockTraffic[i].NumDrops > maxUint32 ||
				d.BlockMetadata[0].BlockList[i].Timestamp-lastTimestamp > maxUint32 {
				return ErrExceedsEncodingSize
			}

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

// Path returns the path of the GPDir (up to the timestamp and including a potential metadata suffix)
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

	defer func() {
		d.isOpen = false
	}()

	// Close all open GPFiles
	var errs []error
	for i := range int(types.ColIdxCount) {
		if d.gpFiles[i] != nil {
			if err := d.gpFiles[i].Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Ensure resources are marked for cleanup
	defer func() {
		d.Metadata.BlockTraffic = nil
		for i := range int(types.ColIdxCount) {
			d.Metadata.BlockMetadata[i].BlockList = nil
			d.Metadata.BlockMetadata[i] = nil
		}
	}()

	// If there was an error writing _any_ of the column files we abort
	// in order to avoid metadata with incorrect / broken information being
	// written to disk (allowing for recovery on the next write)
	if len(errs) > 0 {
		return fmt.Errorf("errors encountered during write of GPFile(s): %v", errs)
	}

	// In write mode, update the metadata on disk (creating / overwriting)
	if d.accessMode == ModeWrite {
		return d.writeMetadataAtomic()
	}

	return nil
}

// Column returns the underlying GPFile for a specified column (lazy-access)
func (d *GPDir) Column(colIdx types.ColumnIndex) (*GPFile, error) {

	if !d.isOpen {
		return nil, ErrDirNotOpen
	}

	if d.gpFiles[colIdx] == nil {
		var err error
		if d.gpFiles[colIdx], err = New(filepath.Join(d.dirPath, types.ColumnFileNames[colIdx]+FileSuffix), d.BlockMetadata[colIdx], d.accessMode, d.options...); err != nil {
			return nil, err
		}
	}

	return d.gpFiles[colIdx], nil
}

// createIfRequired created the underlying path structure (if missing)
func (d *GPDir) createIfRequired() error {
	return os.MkdirAll(d.dirPath, calculateDirPerm(d.permissions))
}

func (d *GPDir) writeMetadataAtomic() error {

	// Create a temporary file (in the destinantion directory to avoid moving accross the FS barrier)
	tempFile, err := os.CreateTemp(d.dirPath, ".tmp-metadata-*")
	if err != nil {
		return err
	}
	defer func() {
		if cerr := os.Remove(tempFile.Name()); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// Serialize the metadata and flush / close the temporary file
	if err = d.Marshal(tempFile); err != nil {
		return err
	}
	if err = tempFile.Close(); err != nil {
		return err
	}

	// Set permissions / file mode
	if err = os.Chmod(tempFile.Name(), d.permissions); err != nil {
		return err
	}

	// Move the temporary file
	if err = os.Rename(tempFile.Name(), d.metaPath); err != nil {
		return err
	}

	// Move / rename the output directory (if suffix has changed)
	if curDirPath, newDirPath := d.dirPath, d.dirTimestampPath+d.Metadata.MarshalString(); curDirPath != newDirPath {
		return os.Rename(curDirPath, newDirPath)
	}
	return nil
}

func (d *GPDir) setPermissions(permissions fs.FileMode) {
	d.permissions = permissions
}

func (d *GPDir) setMetadataFromSuffix(metadataSuffix string) {
	meta := new(Metadata) // no need to use newMetadata() since no block information is used
	if err := meta.UnmarshalString(metadataSuffix); err == nil {
		d.Metadata = meta
	}
}

// DirTimestamp returns timestamp rounded down to the nearest directory time frame (usually a day)
func DirTimestamp(timestamp int64) int64 {
	return (timestamp / EpochDay) * EpochDay
}

func genWritePathForTimestamp(basePath string, timestamp int64) (string, string) {
	dayTimestamp := DirTimestamp(timestamp)
	dayUnix := time.Unix(dayTimestamp, 0)

	searchPath := filepath.Join(basePath, strconv.Itoa(dayUnix.Year()), padNumber(int64(dayUnix.Month())))
	prefix := strconv.FormatInt(dayTimestamp, 10)

	initialDirPath := filepath.Join(searchPath, prefix)
	dirents, err := os.ReadDir(searchPath)
	if err != nil {
		return initialDirPath, initialDirPath
	}

	// Find a matching directory using prefix-based binary search
	if match, found := binarySearchPrefix(dirents, prefix); found {
		return initialDirPath, filepath.Join(searchPath, match)
	}

	return initialDirPath, initialDirPath
}

// genReadPathForTimestamp provides a unified generator method that allows to construct the path to
// the data on disk based on a base path, a timestamp and a metadata suffix
func genReadPathForTimestamp(basePath string, timestamp int64, metadataSuffix string) (string, string) {
	dayTimestamp := DirTimestamp(timestamp)
	dayUnix := time.Unix(dayTimestamp, 0)

	if metadataSuffix == "" {
		path := filepath.Join(basePath, strconv.Itoa(dayUnix.Year()), padNumber(int64(dayUnix.Month())), strconv.FormatInt(dayTimestamp, 10))
		return path, filepath.Join(basePath, strconv.Itoa(dayUnix.Year()), padNumber(int64(dayUnix.Month())), strconv.FormatInt(dayTimestamp, 10))
	}

	path := filepath.Join(basePath, strconv.Itoa(dayUnix.Year()), padNumber(int64(dayUnix.Month())), strconv.FormatInt(dayTimestamp, 10))
	return path, path + "_" + metadataSuffix
}

// recoverDirPath attempts to recover the full path of the GPDir based on the month path and a metadata file
// in case the GPDir path + metadata suffix has changed
func (d *GPDir) recoverDirPath() error {
	searchDir, prefix := filepath.Dir(d.dirMonthPath), filepath.Base(d.dirMonthPath)

	dirEnts, err := os.ReadDir(searchDir)
	if err != nil {
		return fmt.Errorf("failed to list contents of directory `%s`: %w", searchDir, err)
	}

	match, found := binarySearchPrefix(dirEnts, prefix)
	if !found {
		return fmt.Errorf("metadata file `%s` missing", d.MetadataPath())
	}

	d.dirPath = filepath.Join(searchDir, match)
	d.metaPath = filepath.Join(d.dirPath, metadataFileName)

	_, metadataSuffix, err := ExtractTimestampMetadataSuffix(match)
	if err != nil {
		return fmt.Errorf("failed to extract metadata suffix from `%s`: %w", match, err)
	}

	d.setMetadataFromSuffix(metadataSuffix)
	return nil
}

func padNumber(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

func calculateDirPerm(filePerm os.FileMode) os.FileMode {

	if filePerm&0400 != 0 {
		filePerm |= 0100
	}
	if filePerm&0040 != 0 {
		filePerm |= 0010
	}
	if filePerm&0004 != 0 {
		filePerm |= 0001
	}

	return filePerm
}

// This method provides a simple & fast custom prefix-based binary search for a []fs.DirEntry slice
// in order to avoid having to jump through hoops and take the performance penalty of using the
// standard sort facilities for this custom type
func binarySearchPrefix(slice []fs.DirEntry, prefix string) (match string, found bool) {

	if prefix == "" {
		return
	}

	low := 0
	high := len(slice) - 1
	for low <= high {
		mid := (low + high) / 2
		elem := slice[mid].Name()
		if strings.HasPrefix(elem, prefix) {
			return elem, true
		} else if elem < prefix {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	return
}
