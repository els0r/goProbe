/////////////////////////////////////////////////////////////////////////////////
//
// instrument_test.go
//
// Written by Lorenz Breidenbach lob@open.ch, September 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package node

import (
	"reflect"
	"testing"

	"github.com/els0r/goProbe/pkg/types"
)

var IPStringToBytesTests = []struct {
	input   string
	outIP   []byte
	success bool
}{
	{"1.2.3.4", []byte{1, 2, 3, 4}, true},
	{"300.1.2.3", nil, false},
	{"1122:3344:5566:7788:99AA:BBCC:DDEE:FF31", []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x31}, true},
	{"1122:3344:5566::BBCC:DDEE:FF31", []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0, 0, 0, 0, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x31}, true},
}

func TestIpStringToBytes(t *testing.T) {
	for _, test := range IPStringToBytesTests {
		outIP, _, err := types.IPStringToBytes(test.input)
		if !test.success {
			if err == nil {
				t.Fatalf("IPStringToBytes is expected to fail on input %v but it didn't. Instead it output %v",
					test.input, outIP)
			}
		} else {
			if err != nil {
				t.Fatalf("IPStringToBytes unexpectedly failed on input %v. The error is: %s",
					test.input, err)
			}
			if !reflect.DeepEqual(test.outIP, outIP) {
				t.Fatalf("IPStringToBytes returned an unexpected output. Expected output: %v. Actual output: %v",
					test.outIP, outIP)
			}
		}
	}
}

var conditionBytesAndNetmaskTests = []struct {
	input      conditionNode
	outBytes   []byte
	outNetmask int
	ipVersion  types.IPVersion
	success    bool
}{
	// valid ipv4
	{conditionNode{attribute: "sip", comparator: "=", value: "192.168.178.1"}, []byte{192, 168, 178, 1}, 0, types.IPVersionV4, true},
	{conditionNode{attribute: "dip", comparator: "=", value: "192.168.178.1"}, []byte{192, 168, 178, 1}, 0, types.IPVersionV4, true},
	// wrong attribute
	{conditionNode{attribute: "dport", comparator: "=", value: "192.168.178.1"}, nil, 0, types.IPVersionV4, false},
	{conditionNode{attribute: "proto", comparator: "=", value: "192.168.178.1"}, nil, 0, types.IPVersionV4, false},
	{conditionNode{attribute: "snet", comparator: "=", value: "192.168.178.1"}, nil, 0, types.IPVersionV4, false},
	{conditionNode{attribute: "dnet", comparator: "=", value: "192.168.178.1"}, nil, 0, types.IPVersionV4, false},
	// invalid ipv4
	{conditionNode{attribute: "sip", comparator: "=", value: "192.168.178.2221"}, nil, 0, types.IPVersionNone, false},

	// valid ipv6
	{conditionNode{attribute: "sip", comparator: "=", value: "fe80::12"}, []byte{0xFE, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x12}, 0, types.IPVersionV6, true},
	{conditionNode{attribute: "dip", comparator: "=", value: "fe80::12"}, []byte{0xFE, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x12}, 0, types.IPVersionV6, true},
	// wrong attribute
	{conditionNode{attribute: "dport", comparator: "=", value: "fe80::12"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "proto", comparator: "=", value: "fe80::12"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "snet", comparator: "=", value: "fe80::12"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "dnet", comparator: "=", value: "fe80::12"}, nil, 0, types.IPVersionNone, false},
	// invalid ipv6
	{conditionNode{attribute: "sip", comparator: "=", value: "fe80:::2"}, nil, 0, types.IPVersionNone, false},

	// valid CIDR
	{conditionNode{attribute: "snet", comparator: "=", value: "255.168.178.1/0"}, []byte{0, 0, 0, 0}, 0, types.IPVersionV4, true},
	{conditionNode{attribute: "snet", comparator: "=", value: "255.168.178.1/1"}, []byte{128, 0, 0, 0}, 1, types.IPVersionV4, true},
	{conditionNode{attribute: "snet", comparator: "=", value: "255.168.178.1/8"}, []byte{255, 0, 0, 0}, 8, types.IPVersionV4, true},
	{conditionNode{attribute: "dnet", comparator: "=", value: "255.255.255.1/13"}, []byte{255, 248, 0, 0}, 13, types.IPVersionV4, true},
	{conditionNode{attribute: "dnet", comparator: "=", value: "255.255.255.255/32"}, []byte{255, 255, 255, 255}, 32, types.IPVersionV4, true},
	{conditionNode{attribute: "snet", comparator: "=", value: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/0"}, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, 0, types.IPVersionV6, true},
	{conditionNode{attribute: "snet", comparator: "=", value: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/47"}, []byte{255, 255, 255, 255, 255, 254, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, 47, types.IPVersionV6, true},
	{conditionNode{attribute: "snet", comparator: "=", value: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/64"}, []byte{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}, 64, types.IPVersionV6, true},
	{conditionNode{attribute: "snet", comparator: "=", value: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/128"}, []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}, 128, types.IPVersionV6, true},
	// wrong attribute
	{conditionNode{attribute: "sip", comparator: "=", value: "10.0.0.0/16"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "dip", comparator: "=", value: "10.0.0.0/16"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "dport", comparator: "=", value: "fe80::2e/16"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "proto", comparator: "=", value: "::/16"}, nil, 0, types.IPVersionNone, false},
	// invalid CIDR
	{conditionNode{attribute: "dnet", comparator: "=", value: "255.255.255.255/38"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "snet", comparator: "=", value: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/129"}, nil, 0, types.IPVersionNone, false},

	// valid proto
	{conditionNode{attribute: "proto", comparator: "=", value: "119"}, []byte{119}, 0, types.IPVersionNone, true},
	{conditionNode{attribute: "proto", comparator: "=", value: "srp"}, []byte{119}, 0, types.IPVersionNone, true},
	// wrong attribute
	{conditionNode{attribute: "sip", comparator: "=", value: "8"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "dip", comparator: "=", value: "8"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "snet", comparator: "=", value: "8"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "dnet", comparator: "=", value: "8"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "sip", comparator: "=", value: "srp"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "dip", comparator: "=", value: "srp"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "snet", comparator: "=", value: "srp"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "dnet", comparator: "=", value: "srp"}, nil, 0, types.IPVersionNone, false},
	// invalid proto
	{conditionNode{attribute: "proto", comparator: "=", value: "8080"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "proto", comparator: "=", value: "crap"}, nil, 0, types.IPVersionNone, false},
	// TODO(lob): not a valid proto id, ask lenny whether a check should be included
	//{conditionNode{attribute: "proto", comparator: "=", value: "139"}, nil, 0, false},

	// valid port
	{conditionNode{attribute: "dport", comparator: "=", value: "0"}, []byte{0, 0}, 0, types.IPVersionNone, true},
	{conditionNode{attribute: "dport", comparator: "=", value: "80"}, []byte{0, 80}, 0, types.IPVersionNone, true},
	{conditionNode{attribute: "dport", comparator: "=", value: "8080"}, []byte{0x1F, 0x90}, 0, types.IPVersionNone, true},
	{conditionNode{attribute: "dport", comparator: "=", value: "65535"}, []byte{0xFF, 0xFF}, 0, types.IPVersionNone, true},
	// wrong attribute
	{conditionNode{attribute: "sip", comparator: "=", value: "8080"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "dip", comparator: "=", value: "8080"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "snet", comparator: "=", value: "8080"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "dnet", comparator: "=", value: "8080"}, nil, 0, types.IPVersionNone, false},
	// invalid port
	{conditionNode{attribute: "dport", comparator: "=", value: "65536"}, nil, 0, types.IPVersionNone, false},
	{conditionNode{attribute: "dport", comparator: "=", value: "-1"}, nil, 0, types.IPVersionNone, false},

	// wrong attribute
	{conditionNode{attribute: "proto", comparator: "=", value: "leagueoflegends"}, nil, 0, types.IPVersionNone, false},
}

func TestConditionBytesAndNetmask(t *testing.T) {
	for _, test := range conditionBytesAndNetmaskTests {
		bytes, netmask, ipVersion, err := conditionBytesAndNetmask(test.input)
		if !test.success {
			if err == nil {
				t.Fatalf("Expected to fail on input %v but it didn't. Instead it output %v, %v",
					test.input, bytes, netmask)
			}
		} else {
			if err != nil {
				t.Fatalf("Unexpectedly failed on input %v. The error is: %s",
					test.input, err)
			}
			if !reflect.DeepEqual(test.outBytes, bytes) || test.outNetmask != netmask || test.ipVersion != ipVersion {
				t.Fatalf("Returned an unexpected output for input `%s`. Expected output: %v, %v (IPv%d). Actual output: %v, %v (IPv%d)",
					test.input, test.outBytes, test.outNetmask, test.ipVersion, bytes, netmask, ipVersion)
			}
		}
	}
}
