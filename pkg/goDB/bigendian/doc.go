/////////////////////////////////////////////////////////////////////////////////
//
// doc.go
//
// Written by Lorenz Breidenbach lob@open.ch, November 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// Package bigendian provides functionality to convert (u)ints encoded in
// big-endian to little-endian. Note that all architectures officially
// supported by go (x86, amd64, arm) are little-endian. (ARM supports
// big-endian in principle, but go doesn't support big-endian ARM.
// See https://github.com/golang/go/issues/11079 .)
//
// We only have assembler code for amd64, but we have a reference implementation
// in pure go that is used for testing and on non-amd64 platforms.
package bigendian
