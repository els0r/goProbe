package main

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	jsoniter "github.com/json-iterator/go"
)

// LegacyFileSet denotes a collection of all files required to read / parse a legacy DB directory
type LegacyFileSet struct {
	sipFile   *LegacyGPFile
	dipFile   *LegacyGPFile
	dportFile *LegacyGPFile
	protoFile *LegacyGPFile

	bytesRcvdFile *LegacyGPFile
	bytesSentFile *LegacyGPFile
	pktsRcvdFile  *LegacyGPFile
	pktsSentFile  *LegacyGPFile
}

// NewLegacyFileSet instantiates a new legacy DB file set
func NewLegacyFileSet(path string) (*LegacyFileSet, error) {
	var (
		err     error
		fileSet LegacyFileSet
	)

	fileSet.sipFile, err = NewLegacyGPFile(filepath.Join(path, "sip.gpf"))
	if err != nil {
		return nil, err
	}
	fileSet.dipFile, err = NewLegacyGPFile(filepath.Join(path, "dip.gpf"))
	if err != nil {
		return nil, err
	}
	fileSet.dportFile, err = NewLegacyGPFile(filepath.Join(path, "dport.gpf"))
	if err != nil {
		return nil, err
	}
	fileSet.protoFile, err = NewLegacyGPFile(filepath.Join(path, "proto.gpf"))
	if err != nil {
		return nil, err
	}
	fileSet.bytesRcvdFile, err = NewLegacyGPFile(filepath.Join(path, "bytes_rcvd.gpf"))
	if err != nil {
		return nil, err
	}
	fileSet.bytesSentFile, err = NewLegacyGPFile(filepath.Join(path, "bytes_sent.gpf"))
	if err != nil {
		return nil, err
	}
	fileSet.pktsRcvdFile, err = NewLegacyGPFile(filepath.Join(path, "pkts_rcvd.gpf"))
	if err != nil {
		return nil, err
	}
	fileSet.pktsSentFile, err = NewLegacyGPFile(filepath.Join(path, "pkts_sent.gpf"))
	if err != nil {
		return nil, err
	}

	return &fileSet, nil
}

// Close closes a legacy DB file set
func (l LegacyFileSet) Close() error {
	var errs []error
	if err := l.sipFile.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := l.dipFile.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := l.dportFile.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := l.protoFile.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := l.bytesRcvdFile.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := l.bytesSentFile.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := l.pktsRcvdFile.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := l.pktsSentFile.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close one or more files: %v", errs)
	}
	return nil
}

// GetTimestamps returns all timestamps of a legacy DB file set
func (l LegacyFileSet) GetTimestamps() ([]int64, error) {
	return l.bytesRcvdFile.timestamps, nil
}

func (l LegacyFileSet) getBlock(f *LegacyGPFile, ts int64) ([]byte, error) {
	block, err := f.ReadTimedBlock(ts)
	if err != nil {
		return nil, err
	}

	// Cut off the now unneccessary block prefix / suffix
	block = block[8 : len(block)-8]

	return block, nil
}

// GetBlock extracts a specific block from a legacy DB file set
func (l LegacyFileSet) GetBlock(ts int64) (*hashmap.AggFlowMap, error) {
	data := hashmap.NewAggFlowMap()

	sipBlock, err := l.getBlock(l.sipFile, ts)
	if err != nil {
		return nil, err
	}
	dipBlock, err := l.getBlock(l.dipFile, ts)
	if err != nil {
		return nil, err
	}
	dportBlock, err := l.getBlock(l.dportFile, ts)
	if err != nil {
		return nil, err
	}
	protoBlock, err := l.getBlock(l.protoFile, ts)
	if err != nil {
		return nil, err
	}

	bytesRcvdBlock, err := l.getBlock(l.bytesRcvdFile, ts)
	if err != nil {
		return nil, err
	}
	bytesSentBlock, err := l.getBlock(l.bytesSentFile, ts)
	if err != nil {
		return nil, err
	}
	pktsRcvdBlock, err := l.getBlock(l.pktsRcvdFile, ts)
	if err != nil {
		return nil, err
	}
	pktsSentBlock, err := l.getBlock(l.pktsSentFile, ts)
	if err != nil {
		return nil, err
	}

	if len(sipBlock) != 16*len(protoBlock) ||
		len(dipBlock) != 16*len(protoBlock) ||
		len(dportBlock) != 2*len(protoBlock) {
		panic("should not be here")
	}

	for i := 0; i < len(protoBlock); i++ {
		sip := rawIPToAddr(sipBlock[i*16 : i*16+16])
		dip := rawIPToAddr(dipBlock[i*16 : i*16+16])
		if sip.Is4() != dip.Is4() && !sip.IsUnspecified() {
			logger.Warnf("unexpected source / destination IP v4 / v6 mismatch: %s / %s, skipping entry", sip, dip)
			continue
		}

		var V types.Counters

		V.BytesRcvd = binary.BigEndian.Uint64(bytesRcvdBlock[i*8 : i*8+8])
		V.BytesSent = binary.BigEndian.Uint64(bytesSentBlock[i*8 : i*8+8])
		V.PacketsRcvd = binary.BigEndian.Uint64(pktsRcvdBlock[i*8 : i*8+8])
		V.PacketsSent = binary.BigEndian.Uint64(pktsSentBlock[i*8 : i*8+8])

		isIPv4 := sip.Is4() && dip.Is4()
		data.SetOrUpdate(
			newKeyFromNetIPAddr(sip, dip, dportBlock[i*2:i*2+2], protoBlock[i], isIPv4),
			isIPv4, V.BytesRcvd, V.BytesSent, V.PacketsRcvd, V.PacketsSent)
	}

	return data, nil
}

// MetadataFileName denotes the static filename for the metadata
const MetadataFileName = "meta.json"

// BlockMetadata represents metadata for one database block.
type BlockMetadata struct {
	Timestamp            int64 `json:"timestamp"`
	PcapPacketsReceived  int   `json:"pcap_packets_received"`
	PcapPacketsDropped   int   `json:"pcap_packets_dropped"`
	PcapPacketsIfDropped int   `json:"pcap_packets_if_dropped"`
	PacketsLogged        int   `json:"packets_logged"`

	// As in Summary
	FlowCount uint64 `json:"flowcount"`
	Traffic   uint64 `json:"traffic"`
}

// Metadata for a collection of database blocks.
// By convention all blocks belong the same day.
type Metadata struct {
	Blocks []BlockMetadata `json:"blocks"`
}

// GetBlock returns the block metadata for a given timestamp
func (m *Metadata) GetBlock(ts int64) (BlockMetadata, error) {
	for _, block := range m.Blocks {
		if block.Timestamp == ts {
			return block, nil
		}
	}

	return BlockMetadata{}, fmt.Errorf("cannot find block metadata for timestamp %d", ts)
}

// ReadMetadata reads the metadata from the supplied filepath
func ReadMetadata(path string) (*Metadata, error) {
	var result Metadata

	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if err := jsoniter.NewDecoder(f).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func rawIPToAddr(ip []byte) netip.Addr {
	zeros := numZeros(ip)
	ind := len(ip)
	if zeros == 12 {
		// only read first 4 bytes (IPv4)
		ind = 4
	}
	netIP, ok := netip.AddrFromSlice(ip[:ind])
	if !ok {
		return netip.Addr{}
	}
	return netIP
}

func numZeros(ip []byte) uint8 {
	var numZeros uint8

	// count zeros in order to determine whether the address
	// is IPv4 or IPv6
	for i := 4; i < len(ip); i++ {
		if (ip[i] & 0xFF) == 0x00 {
			numZeros++
		}
	}
	return numZeros
}
