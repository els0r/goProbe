/////////////////////////////////////////////////////////////////////////////////
//
// db_writer.go
//
// Written by Lorenz Breidenbach lob@open.ch, January 2016
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/fako1024/gotools/bitpack"
)

const DefaultPermissions = fs.FileMode(0644)

// DBWriter writes goProbe flows to goDB database files
type DBWriter struct {
	dbpath string
	iface  string

	encoderType  encoders.Type
	encoderLevel int
	permissions  fs.FileMode
}

// NewDBWriter initializes a new DBWriter
func NewDBWriter(dbpath string, iface string, encoderType encoders.Type) (w *DBWriter) {
	return &DBWriter{
		dbpath:      dbpath,
		iface:       iface,
		encoderType: encoderType,
		permissions: DefaultPermissions,
	}
}

// Permissions overrides the default permissions for files / directories in the DB
func (w *DBWriter) Permissions(permissions fs.FileMode) *DBWriter {
	w.permissions = permissions
	return w
}

// EncoderLevel overrides the default encoder / compressor level for files / directories in the DB
func (w *DBWriter) EncoderLevel(level int) *DBWriter {
	w.encoderLevel = level
	return w
}

// Write takes an aggregated flow map and its metadata and writes it to disk for a given timestamp
func (w *DBWriter) Write(flowmap *hashmap.AggFlowMap, captureMeta CaptureMetadata, timestamp int64) error {
	var (
		data   [types.ColIdxCount][]byte
		update gpfile.Stats
		err    error
	)

	dir := gpfile.NewDir(filepath.Join(w.dbpath, w.iface), timestamp, gpfile.ModeWrite, gpfile.WithPermissions(w.permissions), gpfile.WithEncoderTypeLevel(w.encoderType, w.encoderLevel))
	if err = dir.Open(); err != nil {
		return fmt.Errorf("failed to create / open daily directory: %w", err)
	}

	data, update = dbData(w.iface, timestamp, flowmap)
	if err := dir.WriteBlocks(timestamp, gpfile.TrafficMetadata{
		NumV4Entries: update.Traffic.NumV4Entries,
		NumV6Entries: update.Traffic.NumV6Entries,
		NumDrops:     captureMeta.PacketsDropped,
	}, update.Counts, data); err != nil {
		return err
	}

	return dir.Close()
}

// BulkWorkload denotes a set of workloads / writes to perform during WriteBulk()
type BulkWorkload struct {
	FlowMap     *hashmap.AggFlowMap
	CaptureMeta CaptureMetadata
	Timestamp   int64
}

// WriteBulk takes multiple aggregated flow maps and their metadata and writes it to disk for a given timestamp
func (w *DBWriter) WriteBulk(workloads []BulkWorkload, dirTimestamp int64) (err error) {
	var (
		data   [types.ColIdxCount][]byte
		update gpfile.Stats
	)

	dir := gpfile.NewDir(filepath.Join(w.dbpath, w.iface), dirTimestamp, gpfile.ModeWrite, gpfile.WithPermissions(w.permissions), gpfile.WithEncoderTypeLevel(w.encoderType, w.encoderLevel))
	if err = dir.Open(); err != nil {
		return fmt.Errorf("failed to create / open daily directory: %w", err)
	}

	for _, workload := range workloads {
		data, update = dbData(w.iface, workload.Timestamp, workload.FlowMap)
		if err := dir.WriteBlocks(workload.Timestamp, gpfile.TrafficMetadata{
			NumV4Entries: update.Traffic.NumV4Entries,
			NumV6Entries: update.Traffic.NumV6Entries,
			NumDrops:     workload.CaptureMeta.PacketsDropped,
		}, update.Counts, data); err != nil {
			return err
		}
	}

	return dir.Close()
}

func dbData(iface string, timestamp int64, aggFlowMap *hashmap.AggFlowMap) ([types.ColIdxCount][]byte, gpfile.Stats) {
	var dbData [types.ColIdxCount][]byte
	var summUpdate gpfile.Stats

	v4List, v6List := aggFlowMap.Flatten()
	v4List = v4List.Sort()
	v6List = v6List.Sort()
	for i := types.ColumnIndex(0); i < types.ColIdxAttributeCount; i++ {
		columnSizeof := types.ColumnSizeofs[i]
		if columnSizeof == types.IPSizeOf {
			dbData[i] = make([]byte, 0, 4*len(v4List)+16*len(v6List))
		} else {
			dbData[i] = make([]byte, 0, types.ColumnSizeofs[i]*(len(v4List)+len(v6List)))
		}
	}

	// loop through the v4 & v6 flow maps to extract the relevant
	// values into database blocks.
	bytesRcvd, bytesSent, pktsRcvd, pktsSent :=
		make([]uint64, 0, len(v4List)+len(v6List)),
		make([]uint64, 0, len(v4List)+len(v6List)),
		make([]uint64, 0, len(v4List)+len(v6List)),
		make([]uint64, 0, len(v4List)+len(v6List))
	for _, list := range []hashmap.List{v4List, v6List} {
		for _, flow := range list {

			// global counters
			summUpdate.Counts = summUpdate.Counts.Add(flow.Val)

			// counters
			bytesRcvd = append(bytesRcvd, flow.BytesRcvd)
			bytesSent = append(bytesSent, flow.BytesSent)
			pktsRcvd = append(pktsRcvd, flow.PacketsRcvd)
			pktsSent = append(pktsSent, flow.PacketsSent)

			// attributes
			dbData[types.DportColIdx] = append(dbData[types.DportColIdx], flow.GetDport()...)
			dbData[types.ProtoColIdx] = append(dbData[types.ProtoColIdx], flow.GetProto())
			dbData[types.SipColIdx] = append(dbData[types.SipColIdx], flow.GetSip()...)
			dbData[types.DipColIdx] = append(dbData[types.DipColIdx], flow.GetDip()...)
		}
	}

	// Perform bit packing on the counter columns
	dbData[types.BytesRcvdColIdx] = bitpack.Pack(bytesRcvd)
	dbData[types.BytesSentColIdx] = bitpack.Pack(bytesSent)
	dbData[types.PacketsRcvdColIdx] = bitpack.Pack(pktsRcvd)
	dbData[types.PacketsSentColIdx] = bitpack.Pack(pktsSent)

	summUpdate.Traffic.NumV4Entries = uint64(len(v4List))
	summUpdate.Traffic.NumV6Entries = uint64(len(v6List))

	return dbData, summUpdate
}
