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

import (
	"encoding/json"
	"fmt"
)

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
	NBytesRcvd uint64 `json:"bytes_rcvd"`
	NBytesSent uint64 `json:"bytes_sent"`
	NPktsRcvd  uint64 `json:"packets_rcvd"`
	NPktsSent  uint64 `json:"packets_sent"`
}

type AggFlowMap map[Key]*Val

// ATTENTION: apart from the obvious use case, the following methods are used to provide flow information
// via syslog, so don't unnecessarily change the order of the fields.

// print the key as a comma separated attribute list
func (k Key) String() string {
	return fmt.Sprintf("%s,%s,%d,%s",
		RawIpToString(k.Sip[:]),
		RawIpToString(k.Dip[:]),
		int(uint16(k.Dport[0])<<8|uint16(k.Dport[1])),
		GetIPProto(int(k.Protocol)),
	)
}

func (k Key) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			SIP   string `json:"sip"`
			DIP   string `json:"dip"`
			Dport uint16 `json:"dport"`
			Proto string `json:"ip_protocol"`
		}{
			RawIpToString(k.Sip[:]),
			RawIpToString(k.Dip[:]),
			uint16(uint16(k.Dport[0])<<8 | uint16(k.Dport[1])),
			GetIPProto(int(k.Protocol)),
		},
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

func (a AggFlowMap) MarshalJSON() ([]byte, error) {
	var toMarshal []interface{}

	for k, v := range a {
		toMarshal = append(toMarshal,
			struct {
				Attributes Key  `json:"attributes"`
				Counters   *Val `json:"counters"`
			}{k, v},
		)
	}
	return json.Marshal(toMarshal)
}
