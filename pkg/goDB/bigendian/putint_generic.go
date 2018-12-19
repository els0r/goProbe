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

func PutUint64(b []byte, val uint64) {
	putUint64Ref(b, val)
}

func PutInt64(b []byte, val int64) {
	putInt64Ref(b, val)
}
