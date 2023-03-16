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
	"path/filepath"

	"github.com/els0r/goProbe/pkg/goDB/encoder/bitpack"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
)

// DBWriter writes goProbe flows to goDB database files
type DBWriter struct {
	dbpath string
	iface  string

	dayTimestamp int64
	encoderType  encoders.Type
}

// NewDBWriter initializes a new DBWriter
func NewDBWriter(dbpath string, iface string, encoderType encoders.Type) (w *DBWriter) {
	return &DBWriter{dbpath, iface, 0, encoderType}
}

// Write takes an aggregated flow map and its metadata and writes it to disk for a given timestamp
func (w *DBWriter) Write(flowmap *hashmap.AggFlowMap, captureMeta CaptureMetadata, timestamp int64) error {
	var (
		data   [types.ColIdxCount][]byte
		update gpfile.Stats
		err    error
	)

	dir := gpfile.NewDir(filepath.Join(w.dbpath, w.iface), timestamp, gpfile.ModeWrite)
	if err = dir.Open(); err != nil {
		err = fmt.Errorf("Could not create / open daily directory: %w", err)
		return err
	}
	defer dir.Close()

	data, update = dbData(w.iface, timestamp, flowmap)
	if err := dir.WriteBlocks(timestamp, gpfile.TrafficMetadata{
		NumV4Entries: update.Traffic.NumV4Entries,
		NumV6Entries: update.Traffic.NumV6Entries,
		NumDrops:     captureMeta.PacketsDropped,
	}, update.Counts, data); err != nil {
		return err
	}

	return err
}

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

	dir := gpfile.NewDir(filepath.Join(w.dbpath, w.iface), dirTimestamp, gpfile.ModeWrite)
	if err = dir.Open(); err != nil {
		err = fmt.Errorf("Could not create / open daily directory: %w", err)
		return err
	}
	defer dir.Close()

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

	return nil
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
	var bytesRcvd, bytesSent, pktsRcvd, pktsSent []uint64
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
