package storage

import (
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
)

// Block denotes a block of goprobe data
type Block struct {
	Offset      uint64
	Len         uint32
	RawLen      uint32
	EncoderType encoders.Type
}

// IsEmpty checks if the block does not store any data
func (b Block) IsEmpty() bool {
	return b.Len == 0 && b.RawLen == 0
}

// BlockHeader denotes a list of blocks pertaining to a storage backend
type BlockHeader struct {
	BlockList     []BlockAtTime
	CurrentOffset uint64

	blocks map[int64]int // Hidden from user / serialization (on-demand creation)
}

// BlockAtTime denotes a block / timestamp pair for easier iteration
type BlockAtTime struct {
	Timestamp int64
	Block
}

// BlockAtTime returns the block for a given timestamp (if exists)
func (b *BlockHeader) BlockAtTime(ts int64) (Block, bool) {
	idx, found := b.BlockIndex(ts)
	if !found {
		return Block{}, false
	}
	return b.BlockList[idx].Block, true
}

// Blocks returns an ordered list of timestamps / blocks
func (b *BlockHeader) Blocks() []BlockAtTime {
	return b.BlockList
}

// BlocksBefore returns an ordered list all timestamps / blocks that
// occurred before a given timestamp (including ts)
func (b *BlockHeader) BlocksBefore(ts int64) []BlockAtTime {
	for i, block := range b.BlockList {
		if block.Timestamp >= ts {
			return b.BlockList[:i]
		}
	}
	return b.BlockList
}

// BlocksAfter returns an ordered list all timestamps / blocks that
// occurred after a given timestamp (including ts). The second return
// ind indicates the index of the first block that is returned
func (b *BlockHeader) BlocksAfter(ts int64) (blocks []BlockAtTime, ind int) {
	for i, block := range b.BlockList {
		if block.Timestamp >= ts {
			return b.BlockList[i+1:], i + 1
		}
	}
	return []BlockAtTime{}, 0
}

// NBlocks returns the number of blocks
func (b *BlockHeader) NBlocks() int {
	return len(b.BlockList)
}

// BlockIndex returns the index of the block in the BlockList for a given
// timestamp (if exists)
func (b *BlockHeader) BlockIndex(ts int64) (idx int, found bool) {

	// Lazy-create block map if required
	if b.blocks == nil {
		b.populateLookupMap()
	}

	blockIdx, ok := b.blocks[ts]
	return blockIdx, ok
}

// AddBlock adds a new block to the header
func (b *BlockHeader) AddBlock(ts int64, block Block) {

	// Lazy-create block map if required
	if b.blocks == nil {
		b.populateLookupMap()
	}

	// Append to both the list and map of blocks
	b.BlockList = append(b.BlockList, BlockAtTime{
		Timestamp: ts,
		Block:     block,
	})
	b.blocks[ts] = len(b.BlockList) - 1
}

func (b *BlockHeader) populateLookupMap() {
	b.blocks = make(map[int64]int, len(b.BlockList))
	for i := 0; i < len(b.BlockList); i++ {
		b.blocks[b.BlockList[i].Timestamp] = i
	}
}
