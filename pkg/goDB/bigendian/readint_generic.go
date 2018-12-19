/////////////////////////////////////////////////////////////////////////////////
//
// readint_generic.go
//
// Code for non-amd64 architectures
//
// Written by Lorenz Breidenbach lob@open.ch, November 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// +build !amd64

package bigendian

func ReadUint64At(b []byte, idx int) uint64 {
	return readUint64AtRef(b, idx)
}

func ReadInt64At(b []byte, idx int) int64 {
	return readInt64AtRef(b, idx)
}

func UnsafeReadUint64At(b []byte, idx int) uint64 {
	return readUint64AtRef(b, idx)
}

func UnsafeReadInt64At(b []byte, idx int) int64 {
	return readInt64AtRef(b, idx)
}
