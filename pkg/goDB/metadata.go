/////////////////////////////////////////////////////////////////////////////////
//
// metadata.go
//
// Written by Lorenz Breidenbach lob@open.ch, January 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
	"fmt"
	"os"

	jsoniter "github.com/json-iterator/go"
)

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

// NewMetadata creates a new Metdata struct
func NewMetadata() *Metadata {
	return &Metadata{}
}

// GetBlock return the block metadata for a given timestamp
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

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := jsoniter.NewDecoder(f).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// TryReadMetadata attempts to read the given metadata file.
// If an error occurs, a fresh Metadata struct is returned.
func TryReadMetadata(path string) *Metadata {
	meta, err := ReadMetadata(path)
	if err != nil {
		return NewMetadata()
	}
	return meta
}

// WriteMetadata stores the metadata on disk
func WriteMetadata(path string, meta *Metadata) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return jsoniter.NewEncoder(f).Encode(meta)
}
