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

func ReadUint64At(b []byte, idx int) uint64

func ReadInt64At(b []byte, idx int) int64

func UnsafeReadUint64At(b []byte, idx int) uint64

func UnsafeReadInt64At(b []byte, idx int) int64
