/////////////////////////////////////////////////////////////////////////////////
//
// readint_reference.go
//
// Reference code used for testing and architectures on which we don't support
// assembler
//
// Written by Lorenz Breidenbach lob@open.ch, November 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package bigendian

func readUint64Ref(b []byte) uint64 {
	return uint64(b[0])<<56 | uint64(b[1])<<48 |
		uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 |
		uint64(b[6])<<8 | uint64(b[7])
}

func readInt64Ref(b []byte) int64 {
	return int64(b[0])<<56 | int64(b[1])<<48 |
		int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 |
		int64(b[6])<<8 | int64(b[7])
}

func readUint64AtRef(b []byte, idx int) uint64 {
	return readUint64Ref(b[idx*8 : idx*8+8])
}

func readInt64AtRef(b []byte, idx int) int64 {
	return readInt64Ref(b[idx*8 : idx*8+8])
}
