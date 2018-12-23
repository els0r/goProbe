/////////////////////////////////////////////////////////////////////////////////
//
// putint_reference.go
//
// Reference code used for testing and architectures on which we don't support
// assembler
//
// Written by Lorenz Breidenbach lob@open.ch, December 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package bigendian

func putUint64Ref(b []byte, val uint64) {
	b[0] = byte(val >> 56)
	b[1] = byte(val >> 48)
	b[2] = byte(val >> 40)
	b[3] = byte(val >> 32)
	b[4] = byte(val >> 24)
	b[5] = byte(val >> 16)
	b[6] = byte(val >> 8)
	b[7] = byte(val)
}

func putInt64Ref(b []byte, val int64) {
	b[0] = byte(val >> 56)
	b[1] = byte(val >> 48)
	b[2] = byte(val >> 40)
	b[3] = byte(val >> 32)
	b[4] = byte(val >> 24)
	b[5] = byte(val >> 16)
	b[6] = byte(val >> 8)
	b[7] = byte(val)
}
