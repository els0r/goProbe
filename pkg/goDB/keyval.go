/////////////////////////////////////////////////////////////////////////////////
//
// keyval.go
//
// Flow map primitives and their utility functions
//
// Written by Lennart Elsen lel@open.ch
//
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import "fmt"

type Key struct {
	Sip      [16]byte
	Dip      [16]byte
	Dport    [2]byte
	Protocol byte
}

// ExtraKey is a key with extra information
type ExtraKey struct {
	Time  int64
	Iface string
	Key
}

type Val struct {
	NBytesRcvd uint64
	NBytesSent uint64
	NPktsRcvd  uint64
	NPktsSent  uint64
}

type AggFlowMap map[Key]*Val

// ATTENTION: apart from the obvious use case, the following methods are used to provide flow information
// via syslog, so don't unnecessarily change the order of the fields.

// print the key as a comma separated attribute list
func (k Key) String() string {
	return fmt.Sprintf("%s,%s,%d,%s",
		rawIpToString(k.Sip[:]),
		rawIpToString(k.Dip[:]),
		int(uint16(k.Dport[0])<<8|uint16(k.Dport[1])),
		GetIPProto(int(k.Protocol)),
	)
}

func (v *Val) String() string {
	return fmt.Sprintf("%d,%d,%d,%d",
		v.NPktsRcvd,
		v.NPktsSent,
		v.NBytesRcvd,
		v.NBytesSent,
	)
}
