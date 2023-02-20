package gpfile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/els0r/goProbe/pkg/goDB/storage"
	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
)

const (

	// EpochDay is one day in seconds
	EpochDay int64 = 86400

	metadataFileName = "blockmeta.json"
)

type Metadata struct {
	BlockMetadata     [types.ColIdxCount]*storage.BlockHeader
	BlockNumV4Entries map[int64]uint64
}

func newMetadata() *Metadata {
	m := Metadata{
		BlockNumV4Entries: make(map[int64]uint64),
	}
	for i := 0; i < int(types.ColIdxCount); i++ {
		m.BlockMetadata[i] = &storage.BlockHeader{
			Blocks:    make(map[int64]storage.Block),
			BlockList: make([]storage.BlockAtTime, 0),
			Version:   headerVersion,
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
func (d *GPDir) Deserialize(reader io.Reader) error {
	return jsoniter.NewDecoder(reader).Decode(&d.Metadata)
}

// TODO: Customize encoding (deduplicate map / slice info)
func (d *GPDir) Serialize(writer io.Writer) error {
	return jsoniter.NewEncoder(writer).Encode(d.Metadata)
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
