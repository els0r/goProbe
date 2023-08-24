package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"

	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/fako1024/gotools/bitpack"
)

type ModernFileSet struct {
	sipFile   *GPFile
	dipFile   *GPFile
	dportFile *GPFile
	protoFile *GPFile

	bytesRcvdFile *GPFile
	bytesSentFile *GPFile
	pktsRcvdFile  *GPFile
	pktsSentFile  *GPFile
}

func NewModernFileSet(path string) (*ModernFileSet, error) {
	var (
		err     error
		fileSet ModernFileSet
	)

	fileSet.sipFile, err = New(filepath.Join(path, "sip.gpf"), ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.dipFile, err = New(filepath.Join(path, "dip.gpf"), ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.dportFile, err = New(filepath.Join(path, "dport.gpf"), ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.protoFile, err = New(filepath.Join(path, "proto.gpf"), ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.bytesRcvdFile, err = New(filepath.Join(path, "bytes_rcvd.gpf"), ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.bytesSentFile, err = New(filepath.Join(path, "bytes_sent.gpf"), ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.pktsRcvdFile, err = New(filepath.Join(path, "pkts_rcvd.gpf"), ModeRead)
	if err != nil {
		return nil, err
	}
	fileSet.pktsSentFile, err = New(filepath.Join(path, "pkts_sent.gpf"), ModeRead)
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

func (l ModernFileSet) getBlock(f *GPFile, ts int64) ([]byte, error) {
	block, err := f.ReadBlock(ts)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (l ModernFileSet) GetBlock(ts int64) (*hashmap.AggFlowMap, error) {
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

		sip := rawIPToAddr(sipBlock[i*16 : i*16+16])
		dip := rawIPToAddr(dipBlock[i*16 : i*16+16])
		if sip.Is4() != dip.Is4() && !sip.IsUnspecified() {
			logger.Warnf("unexpected source / destination IP v4 / v6 mismatch: %s / %s, skipping entry", sip, dip)
			continue
		}

		var V types.Counters

		// Unpack counters using bit packing if enabled, otherwise just copy them using fixed bit width
		if useBitPacking {
			V.BytesRcvd = bitpack.Uint64At(bytesRcvdBlock, i, byteWidthBytesRcvd)
			V.BytesSent = bitpack.Uint64At(bytesSentBlock, i, byteWidthBytesSent)
			V.PacketsRcvd = bitpack.Uint64At(pktsRcvdBlock, i, byteWidthPktsRcvd)
			V.PacketsSent = bitpack.Uint64At(pktsSentBlock, i, byteWidthPktsSent)
		} else {
			V.BytesRcvd = binary.BigEndian.Uint64(bytesRcvdBlock[i*8 : i*8+8])
			V.BytesSent = binary.BigEndian.Uint64(bytesSentBlock[i*8 : i*8+8])
			V.PacketsRcvd = binary.BigEndian.Uint64(pktsRcvdBlock[i*8 : i*8+8])
			V.PacketsSent = binary.BigEndian.Uint64(pktsSentBlock[i*8 : i*8+8])
		}

		isIPv4 := sip.Is4() && dip.Is4()
		data.SetOrUpdate(
			newKeyFromNetIPAddr(sip, dip, dportBlock[i*2:i*2+2], protoBlock[i], isIPv4),
			isIPv4, V.BytesRcvd, V.BytesSent, V.PacketsRcvd, V.PacketsSent)
	}

	return data, nil
}
