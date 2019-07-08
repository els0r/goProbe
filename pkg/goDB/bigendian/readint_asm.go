/////////////////////////////////////////////////////////////////////////////////
//
// readint_asm.go
//
// Stubs for architectures on which we support assembler.
//
// Written by Lorenz Breidenbach lob@open.ch, November 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// +build amd64

package bigendian

// ReadUint64At returns the uint64 stored at location idx in b
func ReadUint64At(b []byte, idx int) uint64

// ReadInt64At returns the int64 stored at location idx in b
func ReadInt64At(b []byte, idx int) int64

// UnsafeReadUint64At returns the int64 stored at location idx in b. It accesses the data directly via the unsafe package
func UnsafeReadUint64At(b []byte, idx int) uint64

// UnsafeReadInt64At returns the int64 stored at location idx in b. It accesses the data directly via the unsafe package
func UnsafeReadInt64At(b []byte, idx int) int64

func panicIndex() {
	panic("index out of range")
}
