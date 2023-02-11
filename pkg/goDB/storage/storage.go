package storage

import (
	"sort"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
)

// Block denotes a block of goprobe data
type Block struct {
	EncoderType encoders.Type `json:"e,omitempty"`
	Offset      int64         `json:"p,omitempty"`
	Len         int           `json:"l,omitempty"`
	RawLen      int           `json:"r,omitempty"`
}

// IsEmpty checks if the block does not store any data
func (b Block) IsEmpty() bool {
	return b.Len == 0 && b.RawLen == 0
}

// BlockHeader denotes a list of blocks pertaining to a storage backend
type BlockHeader struct {
	Blocks        map[int64]Block `json:"b,omitempty"`
	CurrentOffset int64           `json:"p,omitempty"`
	Version       int             `json:"v"`
}

// BlockAtTime denotes a block / timestamp pair for easier iteration
type BlockAtTime struct {
	Timestamp int64
	Block
}

// OrderedList returns an ordered list of timestamps / blocks
func (b BlockHeader) OrderedList() []BlockAtTime {
	result := make([]BlockAtTime, 0, len(b.Blocks))

	for k, v := range b.Blocks {
		result = append(result, BlockAtTime{
			Timestamp: k,
			Block:     v,
		})
	}

	sort.Slice(result, func(i int, j int) bool {
		return result[i].Timestamp < result[j].Timestamp
	})

	return result
}

// Backend denotes a generic goDB storage backend
type Backend interface {

	// Blocks returns the list of blocks (and its metadata) available on the storage
	Blocks() (BlockHeader, error)

	// ReadBlock searches if a block for a given timestamp exists and returns in its data
	ReadBlock(timestamp time.Time) ([]byte, error)

	// WriteBlock writes data for a given timestamp to storage
	WriteBlock(timestamp time.Time, blockData []byte) error

	// Close closes a storage backend
	Close() error
}
