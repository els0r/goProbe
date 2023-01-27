package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"

	"github.com/els0r/goProbe/pkg/goDB"
)

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

func (l LegacyFileSet) GetBlock(ts int64) (goDB.AggFlowMap, error) {
	data := make(goDB.AggFlowMap)

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

		var K goDB.Key
		var V goDB.Val

		copy(K.Sip[:], sipBlock[i*16:i*16+16])
		copy(K.Dip[:], dipBlock[i*16:i*16+16])
		copy(K.Dport[:], dportBlock[i*2:i*2+2])
		K.Protocol = protoBlock[i]

		V.NBytesRcvd = binary.BigEndian.Uint64(bytesRcvdBlock[i*8 : i*8+8])
		V.NBytesSent = binary.BigEndian.Uint64(bytesSentBlock[i*8 : i*8+8])
		V.NPktsRcvd = binary.BigEndian.Uint64(pktsRcvdBlock[i*8 : i*8+8])
		V.NPktsSent = binary.BigEndian.Uint64(pktsSentBlock[i*8 : i*8+8])

		entry, exists := data[K]
		if exists {
			entry.NBytesRcvd += V.NBytesRcvd
			entry.NBytesSent += V.NBytesSent
			entry.NPktsRcvd += V.NPktsRcvd
			entry.NPktsSent += V.NPktsSent
			data[K] = entry
		} else {
			data[K] = &V
		}
	}

	return data, nil
}
