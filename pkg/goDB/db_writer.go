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
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/encoder/bitpack"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
)

const (
	// QueryLogFile is the name of the query log written by the query package
	QueryLogFile = "query.log"
	// MetadataFileName specifies the location of the daily column metadata file
	MetadataFileName = "meta.json"
)

// DBWriter writes goProbe flows to goDB database files
type DBWriter struct {
	dbpath string
	iface  string

	dayTimestamp int64
	encoderType  encoders.Type

	metadata *Metadata
}

// NewDBWriter initializes a new DBWriter
func NewDBWriter(dbpath string, iface string, encoderType encoders.Type) (w *DBWriter) {
	return &DBWriter{dbpath, iface, 0, encoderType, new(Metadata)}
}

func (w *DBWriter) dailyDir(timestamp int64) (path string) {
	dailyDir := strconv.FormatInt(gpfile.DirTimestamp(timestamp), 10)
	path = filepath.Join(w.dbpath, w.iface, dailyDir)
	return
}

// TODO: Merge with GPDir metadata
func (w *DBWriter) writeMetadata(timestamp int64, meta BlockMetadata) error {
	if w.dayTimestamp != gpfile.DirTimestamp(timestamp) {
		w.metadata = nil
		w.dayTimestamp = gpfile.DirTimestamp(timestamp)
	}

	path := filepath.Join(w.dailyDir(timestamp), MetadataFileName)

	if w.metadata == nil {
		w.metadata = TryReadMetadata(path)
	}

	w.metadata.Blocks = append(w.metadata.Blocks, meta)

	return WriteMetadata(path, w.metadata)
}

// func (w *DBWriter) writeBlock(timestamp int64, column string, data []byte) error {
// 	path := filepath.Join(w.dailyDir(timestamp), column+".gpf")
// 	gpfile, err := gpfile.New(path, gpfile.ModeWrite, gpfile.WithEncoder(w.encoderType))
// 	if err != nil {
// 		return err
// 	}
// 	defer gpfile.Close()

// 	if err := gpfile.WriteBlock(timestamp, data); err != nil {
// 		return err
// 	}

// 	return nil
// }

func (w *DBWriter) createQueryLog() error {
	var (
		err     error
		logfile *os.File
	)
	qlogPath := filepath.Join(w.dbpath, QueryLogFile)
	logfile, err = os.OpenFile(qlogPath, os.O_CREATE, 0666)
	if err != nil {
		err = fmt.Errorf("failed to create query log: %s", err)
		return err
	}
	logfile.Close()
	err = os.Chmod(qlogPath, 0666)
	if err != nil {
		err = fmt.Errorf("failed to set query log permissions: %s", err)
		return err
	}
	return nil
}

// Write takes an aggregated flow map and its metadata and writes it to disk for a given timestamp
func (w *DBWriter) Write(flowmap *hashmap.AggFlowMap, meta BlockMetadata, timestamp int64) (InterfaceSummaryUpdate, error) {
	var (
		data   [types.ColIdxCount][]byte
		update InterfaceSummaryUpdate
		err    error
	)

	dir := gpfile.NewDir(filepath.Join(w.dbpath, w.iface), timestamp, gpfile.ModeWrite)
	if err = dir.Open(); err != nil {
		err = fmt.Errorf("Could not create / open daily directory: %w", err)
		return update, err
	}
	defer dir.Close()

	// check if the query log exists and create it if necessary
	err = w.createQueryLog()
	if err != nil {
		return update, err
	}

	data, update = dbData(w.iface, timestamp, flowmap)
	if err := dir.WriteBlocks(timestamp, update.NumIPV4Entries, data); err != nil {
		return update, err
	}

	meta.FlowCount = update.FlowCount
	meta.Traffic = update.Traffic

	if err = w.writeMetadata(timestamp, meta); err != nil {
		return update, err
	}

	return update, err
}

type BulkWorkload struct {
	FlowMap   *hashmap.AggFlowMap
	Meta      BlockMetadata
	Timestamp int64
}

// WriteBulk takes multiple aggregated flow maps and their metadata and writes it to disk for a given timestamp
func (w *DBWriter) WriteBulk(workloads []BulkWorkload, dirTimestamp int64) (err error) {
	var (
		data   [types.ColIdxCount][]byte
		update InterfaceSummaryUpdate
	)

	metaDataPath := filepath.Join(w.dailyDir(dirTimestamp), MetadataFileName)
	if w.metadata == nil {
		w.metadata = TryReadMetadata(metaDataPath)
	}

	dir := gpfile.NewDir(filepath.Join(w.dbpath, w.iface), dirTimestamp, gpfile.ModeWrite)
	if err = dir.Open(); err != nil {
		err = fmt.Errorf("Could not create / open daily directory: %w", err)
		return err
	}
	defer dir.Close()

	// check if the query log exists and create it if necessary
	err = w.createQueryLog()
	if err != nil {
		return err
	}

	for _, workload := range workloads {
		data, update = dbData(w.iface, workload.Timestamp, workload.FlowMap)
		if err := dir.WriteBlocks(workload.Timestamp, update.NumIPV4Entries, data); err != nil {
			return err
		}
		workload.Meta.FlowCount = update.FlowCount
		workload.Meta.Traffic = update.Traffic
		w.metadata.Blocks = append(w.metadata.Blocks, workload.Meta)
	}

	return WriteMetadata(metaDataPath, w.metadata)
}

func dbData(iface string, timestamp int64, aggFlowMap *hashmap.AggFlowMap) ([types.ColIdxCount][]byte, InterfaceSummaryUpdate) {
	var dbData [types.ColIdxCount][]byte
	summUpdate := new(InterfaceSummaryUpdate)

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

	summUpdate.Timestamp = time.Unix(timestamp, 0)
	summUpdate.Interface = iface

	// loop through the v4 & v6 flow maps to extract the relevant
	// values into database blocks.
	var bytesRcvd, bytesSent, pktsRcvd, pktsSent []uint64
	for _, list := range []hashmap.List{v4List, v6List} {
		for _, flow := range list {

			// global counters
			summUpdate.FlowCount++
			summUpdate.Traffic += flow.BytesRcvd
			summUpdate.Traffic += flow.BytesSent

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

	summUpdate.NumIPV4Entries = uint64(len(v4List))

	return dbData, *summUpdate
}
