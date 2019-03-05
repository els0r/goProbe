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

// Key stores the 5-tuple which defines a goProbe flow
type Key struct {
	Sip      [16]byte
	Dip      [16]byte
	Dport    [2]byte
	Protocol byte
}

// ExtraKey is a Key with time and interface information
type ExtraKey struct {
	Time  int64
	Iface string
	Key
}

// Val stores the goProbe flow counters
type Val struct {
	NBytesRcvd uint64 `json:"bytes_rcvd"`
	NBytesSent uint64 `json:"bytes_sent"`
	NPktsRcvd  uint64 `json:"packets_rcvd"`
	NPktsSent  uint64 `json:"packets_sent"`
}

// AggFlowMap stores all flows where the source port from the FlowLog has been aggregated
type AggFlowMap map[Key]*Val

// ATTENTION: apart from the obvious use case, the following methods are used to provide flow information
// via syslog, so don't unnecessarily change the order of the fields.

// String prints the key as a comma separated attribute list
func (k Key) String() string {
	return fmt.Sprintf("%s,%s,%d,%s",
		RawIPToString(k.Sip[:]),
		RawIPToString(k.Dip[:]),
		int(uint16(k.Dport[0])<<8|uint16(k.Dport[1])),
		GetIPProto(int(k.Protocol)),
	)
}

// MarshalJSON implements the Marshaler interface
func (k Key) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			SIP   string `json:"sip"`
			DIP   string `json:"dip"`
			Dport uint16 `json:"dport"`
			Proto string `json:"ip_protocol"`
		}{
			RawIPToString(k.Sip[:]),
			RawIPToString(k.Dip[:]),
			uint16(uint16(k.Dport[0])<<8 | uint16(k.Dport[1])),
			GetIPProto(int(k.Protocol)),
		},
	)
}

// String prints the comma-seperated flow counters
func (v *Val) String() string {
	return fmt.Sprintf("%d,%d,%d,%d",
		v.NPktsRcvd,
		v.NPktsSent,
		v.NBytesRcvd,
		v.NBytesSent,
	)
}

// MarshalJSON implements the Marshaler interface for the whole flow map
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
