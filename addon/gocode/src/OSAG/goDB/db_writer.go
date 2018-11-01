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

	"OSAG/goDB/bigendian"
)

const (
	// Used for compression applied by GPFile.
	COMPRESSION_LEVEL = 512

	METADATA_FILE_NAME = "meta.json"
)

// DayTimestamp returns timestamp rounded down to the nearest day
func DayTimestamp(timestamp int64) int64 {
	return (timestamp / EPOCH_DAY) * EPOCH_DAY
}

type DBWriter struct {
	dbpath string
	iface  string

	dayTimestamp int64

	metadata *Metadata
}

func NewDBWriter(dbpath string, iface string) (w *DBWriter) {
	return &DBWriter{
		dbpath,
		iface,

		0,

		new(Metadata),
	}
}

func (w *DBWriter) dailyDir(timestamp int64) (path string) {
	dailyDir := strconv.FormatInt(DayTimestamp(timestamp), 10)
	path = filepath.Join(w.dbpath, w.iface, dailyDir)
	return
}

func (w *DBWriter) writeMetadata(timestamp int64, meta BlockMetadata) error {
	if w.dayTimestamp != DayTimestamp(timestamp) {
		w.metadata = nil
		w.dayTimestamp = DayTimestamp(timestamp)
	}

	path := filepath.Join(w.dailyDir(timestamp), METADATA_FILE_NAME)

	if w.metadata == nil {
		w.metadata = TryReadMetadata(path)
	}

	w.metadata.Blocks = append(w.metadata.Blocks, meta)

	return WriteMetadata(path, w.metadata)
}

func (w *DBWriter) writeBlock(timestamp int64, column string, data []byte) error {
	path := filepath.Join(w.dailyDir(timestamp), column+".gpf")
	gpfile, err := NewGPFile(path)
	if err != nil {
		return err
	}
	defer gpfile.Close()

	if err := gpfile.WriteTimedBlock(timestamp, data, COMPRESSION_LEVEL); err != nil {
		return err
	}

	return nil
}

func (w *DBWriter) Write(flowmap AggFlowMap, meta BlockMetadata, timestamp int64) (InterfaceSummaryUpdate, error) {
	var (
		dbdata [COLIDX_COUNT][]byte
		update InterfaceSummaryUpdate
		err    error
	)

	if err = os.MkdirAll(w.dailyDir(timestamp), 0755); err != nil {
		err = fmt.Errorf("Could not create daily directory: %s", err.Error())
		return update, err
	}

	dbdata, update = dbData(w.iface, timestamp, flowmap)

	for i := columnIndex(0); i < COLIDX_COUNT; i++ {
		if err = w.writeBlock(timestamp, columnFileNames[i], dbdata[i]); err != nil {
			return update, err
		}
	}

	meta.FlowCount = update.FlowCount
	meta.Traffic = update.Traffic

	if err = w.writeMetadata(timestamp, meta); err != nil {
		return update, err
	}

	return update, err
}

func dbData(iface string, timestamp int64, aggFlowMap AggFlowMap) ([COLIDX_COUNT][]byte, InterfaceSummaryUpdate) {
	var dbData [COLIDX_COUNT][]byte
	summUpdate := new(InterfaceSummaryUpdate)

	for i := columnIndex(0); i < COLIDX_COUNT; i++ {
		// size: initial timestamp + values + final timestamp
		size := 8 + columnSizeofs[i]*len(aggFlowMap) + 8
		dbData[i] = make([]byte, 0, size)
	}

	summUpdate.Timestamp = time.Unix(timestamp, 0)
	summUpdate.Interface = iface

	timestampBytes := make([]byte, 8)
	bigendian.PutInt64(timestampBytes, timestamp)

	for i := columnIndex(0); i < COLIDX_COUNT; i++ {
		dbData[i] = append(dbData[i], timestampBytes...)
	}

	counterBytes := make([]byte, 8)

	// loop through the flow map to extract the relevant
	// values into database blocks.
	for K, V := range aggFlowMap {

		summUpdate.FlowCount++
		summUpdate.Traffic += V.NBytesRcvd
		summUpdate.Traffic += V.NBytesSent

		// counters
		bigendian.PutUint64(counterBytes, V.NBytesRcvd)
		dbData[BYTESRCVD_COLIDX] = append(dbData[BYTESRCVD_COLIDX], counterBytes...)

		bigendian.PutUint64(counterBytes, V.NBytesSent)
		dbData[BYTESSENT_COLIDX] = append(dbData[BYTESSENT_COLIDX], counterBytes...)

		bigendian.PutUint64(counterBytes, V.NPktsRcvd)
		dbData[PKTSRCVD_COLIDX] = append(dbData[PKTSRCVD_COLIDX], counterBytes...)

		bigendian.PutUint64(counterBytes, V.NPktsSent)
		dbData[PKTSSENT_COLIDX] = append(dbData[PKTSSENT_COLIDX], counterBytes...)

		// attributes
		dbData[DIP_COLIDX] = append(dbData[DIP_COLIDX], K.Dip[:]...)
		dbData[SIP_COLIDX] = append(dbData[SIP_COLIDX], K.Sip[:]...)
		dbData[DPORT_COLIDX] = append(dbData[DPORT_COLIDX], K.Dport[:]...)
		dbData[PROTO_COLIDX] = append(dbData[PROTO_COLIDX], K.Protocol)
	}

	// push postamble to the arrays
	for i := columnIndex(0); i < COLIDX_COUNT; i++ {
		dbData[i] = append(dbData[i], timestampBytes...)
	}

	return dbData, *summUpdate
}
