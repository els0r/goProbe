package gpfile

import (
	"errors"
	"strings"
	"unsafe"

	"github.com/els0r/goProbe/pkg/goDB/storage"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/fako1024/gotools/bitpack"
)

const (
	metadataFileName = ".blockmeta"
)

// TrafficMetadata denotes a serializable set of metadata information about traffic stats
type TrafficMetadata struct {
	NumV4Entries uint64 `json:"num_v4_entries"`
	NumV6Entries uint64 `json:"num_v6_entries"`
	NumDrops     uint64 `json:"num_drops"`
}

// Stats denotes statistics for a GPDir instance
type Stats struct {
	Counts  types.Counters  `json:"counts"`
	Traffic TrafficMetadata `json:"traffic"`
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

// Add computes the sum of all counters and traffic metadata for the stats
func (s Stats) Add(s2 Stats) Stats {
	s.Counts.Add(s2.Counts)
	s.Traffic = s.Traffic.Add(s2.Traffic)
	return s
}

// Sub computes the sum of all counters and traffic metadata for the stats
func (s Stats) Sub(s2 Stats) Stats {
	s.Counts.Sub(s2.Counts)
	s.Traffic = s.Traffic.Sub(s2.Traffic)
	return s
}

// Metadata denotes a serializable set of metadata (both globally and per-block)
type Metadata struct {
	BlockMetadata [types.ColIdxCount]*storage.BlockHeader
	BlockTraffic  []TrafficMetadata

	Stats
	Version uint64
}

// newMetadata initializes a new Metadata set (internal / serialization use only)
func newMetadata() *Metadata {
	m := Metadata{
		BlockTraffic: make([]TrafficMetadata, 0),
		Version:      headerVersion,
	}
	for i := 0; i < int(types.ColIdxCount); i++ {
		m.BlockMetadata[i] = &storage.BlockHeader{
			CurrentOffset: 0,
			BlockList:     make([]storage.BlockAtTime, 0),
		}
	}
	return &m
}

const (
	maxDirnameLength = 96 // accounts for a 12-digit epoch timestamp and 7 worst-case compressed uint64 values & delimeters

	delimUnderscore = 95 // "_"
	delimDash       = 45 // "-"
)

// MarshalString marshals (partial) metadata information into a (compressed) string
// representation
func (m *Metadata) MarshalString() string {

	buf := make([]byte, maxDirnameLength)
	buf[0] = delimUnderscore

	pos := 1

	n := bitpack.EncodeUint64ToByteBuf(m.Traffic.NumV4Entries, buf[pos:])
	pos += n + 1
	buf[pos-1] = delimDash

	n = bitpack.EncodeUint64ToByteBuf(m.Traffic.NumV6Entries, buf[pos:])
	pos += n + 1
	buf[pos-1] = delimDash

	n = bitpack.EncodeUint64ToByteBuf(m.Traffic.NumDrops, buf[pos:])
	pos += n + 1
	buf[pos-1] = delimDash

	n = bitpack.EncodeUint64ToByteBuf(m.Counts.BytesRcvd, buf[pos:])
	pos += n + 1
	buf[pos-1] = delimDash

	n = bitpack.EncodeUint64ToByteBuf(m.Counts.BytesSent, buf[pos:])
	pos += n + 1
	buf[pos-1] = delimDash

	n = bitpack.EncodeUint64ToByteBuf(m.Counts.PacketsRcvd, buf[pos:])
	pos += n + 1
	buf[pos-1] = delimDash

	n = bitpack.EncodeUint64ToByteBuf(m.Counts.PacketsSent, buf[pos:])
	pos += n

	// Subslice to string length and cast to string (zero-allocation)
	buf = buf[0:pos]
	return *(*string)(unsafe.Pointer(&buf)) // #nosec G103
}

// UnmarshalString deserializes a string representation of (partial) metadata
// into an existing metadata structure
func (m *Metadata) UnmarshalString(input string) error {

	fields := strings.Split(input, "-")
	if len(fields) != 7 {
		return errors.New("invalid number of string fields")
	}

	m.Traffic.NumV4Entries = bitpack.DecodeUint64FromString(fields[0])
	m.Traffic.NumV6Entries = bitpack.DecodeUint64FromString(fields[1])
	m.Traffic.NumDrops = bitpack.DecodeUint64FromString(fields[2])

	m.Counts.BytesRcvd = bitpack.DecodeUint64FromString(fields[3])
	m.Counts.BytesSent = bitpack.DecodeUint64FromString(fields[4])
	m.Counts.PacketsRcvd = bitpack.DecodeUint64FromString(fields[5])
	m.Counts.PacketsSent = bitpack.DecodeUint64FromString(fields[6])

	return nil
}
