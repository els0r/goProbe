/////////////////////////////////////////////////////////////////////////////////
//
// putint_generic.go
//
// Code for all architectures as there is no assembler implementation yet.
//
// Written by Lorenz Breidenbach lob@open.ch, December 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package bigendian

// PutUint64 writes a 64-bit unsigned integer val to b
func PutUint64(b []byte, val uint64) {
	putUint64Ref(b, val)
}

// PutInt64 writes a 64-bit integer val to b
func PutInt64(b []byte, val int64) {
	putInt64Ref(b, val)
}
