package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"

	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/bitpack"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
)

type ModernFileSet struct {
	sipFile   *gpfile.GPFile
	dipFile   *gpfile.GPFile
	dportFile *gpfile.GPFile
	protoFile *gpfile.GPFile

	bytesRcvdFile *gpfile.GPFile
	bytesSentFile *gpfile.GPFile
	pktsRcvdFile  *gpfile.GPFile
	pktsSentFile  *gpfile.GPFile
}

func NewModernFileSet(path string) (*ModernFileSet, error) {
	var (
		err     error
		fileSet ModernFileSet
	)

	fileSet.sipFile, err = gpfile.New(filepath.Join(path, "sip.gpf"), gpfile.ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.dipFile, err = gpfile.New(filepath.Join(path, "dip.gpf"), gpfile.ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.dportFile, err = gpfile.New(filepath.Join(path, "dport.gpf"), gpfile.ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.protoFile, err = gpfile.New(filepath.Join(path, "proto.gpf"), gpfile.ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.bytesRcvdFile, err = gpfile.New(filepath.Join(path, "bytes_rcvd.gpf"), gpfile.ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.bytesSentFile, err = gpfile.New(filepath.Join(path, "bytes_sent.gpf"), gpfile.ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.pktsRcvdFile, err = gpfile.New(filepath.Join(path, "pkts_rcvd.gpf"), gpfile.ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.pktsSentFile, err = gpfile.New(filepath.Join(path, "pkts_sent.gpf"), gpfile.ModeRead)
	if err != nil {
		return nil, err
	}

	return &fileSet, nil
}

func (l ModernFileSet) Close() error {
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

func (l ModernFileSet) GetTimestamps() ([]int64, error) {

	blocks, err := l.sipFile.Blocks()
	if err != nil {
		return nil, err
	}
	var res []int64
	for _, block := range blocks.OrderedList() {
		res = append(res, block.Timestamp)
	}

	return res, nil
}

func (l ModernFileSet) getBlock(f *gpfile.GPFile, ts int64) ([]byte, error) {
	block, err := f.ReadBlock(ts)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (l ModernFileSet) GetBlock(ts int64) (goDB.AggFlowMap, error) {
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

	// Check if the counters were encoded using bit packing
	// Rationale: If they all have a fixed bit width of 8 bytes per item they are not
	useBitPacking := true
	if len(bytesRcvdBlock) == 8*len(protoBlock) &&
		len(bytesSentBlock) == 8*len(protoBlock) &&
		len(pktsRcvdBlock) == 8*len(protoBlock) &&
		len(pktsSentBlock) == 8*len(protoBlock) {
		useBitPacking = false
	}
	var (
		byteWidthBytesRcvd, byteWidthBytesSent, byteWidthPktsRcvd, byteWidthPktsSent int
	)
	if useBitPacking {
		byteWidthBytesRcvd = bitpack.ByteWidth(bytesRcvdBlock)
		byteWidthBytesSent = bitpack.ByteWidth(bytesSentBlock)
		byteWidthPktsRcvd = bitpack.ByteWidth(pktsRcvdBlock)
		byteWidthPktsSent = bitpack.ByteWidth(pktsSentBlock)
	}

	for i := 0; i < len(protoBlock); i++ {

		var K goDB.Key
		var V goDB.Val

		copy(K.Sip[:], sipBlock[i*16:i*16+16])
		copy(K.Dip[:], dipBlock[i*16:i*16+16])
		copy(K.Dport[:], dportBlock[i*2:i*2+2])
		K.Protocol = protoBlock[i]

		// Unpack counters using bit packing if enabled, otherwise just copy them using fixed bit width
		if useBitPacking {
			V.NBytesRcvd = bitpack.Uint64At(bytesRcvdBlock, i, byteWidthBytesRcvd)
			V.NBytesSent = bitpack.Uint64At(bytesSentBlock, i, byteWidthBytesSent)
			V.NPktsRcvd = bitpack.Uint64At(pktsRcvdBlock, i, byteWidthPktsRcvd)
			V.NPktsSent = bitpack.Uint64At(pktsSentBlock, i, byteWidthPktsSent)
		} else {
			V.NBytesRcvd = binary.BigEndian.Uint64(bytesRcvdBlock[i*8 : i*8+8])
			V.NBytesSent = binary.BigEndian.Uint64(bytesSentBlock[i*8 : i*8+8])
			V.NPktsRcvd = binary.BigEndian.Uint64(pktsRcvdBlock[i*8 : i*8+8])
			V.NPktsSent = binary.BigEndian.Uint64(pktsSentBlock[i*8 : i*8+8])
		}

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
