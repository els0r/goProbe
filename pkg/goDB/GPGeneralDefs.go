/////////////////////////////////////////////////////////////////////////////////
//
// GPGeneralDefs.go
//
// Type definitions and helper functions used throughout this package
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import "net/netip"

// DBData holds all data for the flow attributes and counters
type DBData struct {
	// counters
	BytesRcvd []byte
	BytesSent []byte
	PktsRcvd  []byte
	PktsSent  []byte

	// attributes
	Dip   []byte
	Sip   []byte
	Dport []byte
	Proto []byte

	// metadata (important for folder naming)
	Tstamp int64
	Iface  string
}

// NewDBData returns the DBData struct in case it needs to be set from an external
// go program that included goProbe
func NewDBData(br []byte, bs []byte, pr []byte, ps []byte, dip []byte, sip []byte, dport []byte, proto []byte, tstamp int64, iface string) DBData {
	return DBData{br, bs, pr, ps, dip, sip, dport, proto, tstamp, iface}
}

func RawIPToAddr(ip []byte) netip.Addr {
	zeros := numZeros(ip)
	ind := len(ip)
	if zeros == 12 {
		// only read first 4 bytes (IPv4)
		ind = 4
	}
	netIP, ok := netip.AddrFromSlice(ip[:ind])
	if !ok {
		return netip.Addr{}
	}
	return netIP
}

func numZeros(ip []byte) uint8 {
	var numZeros uint8

	// count zeros in order to determine whether the address
	// is IPv4 or IPv6
	for i := 4; i < len(ip); i++ {
		if (ip[i] & 0xFF) == 0x00 {
			numZeros++
		}
	}
	return numZeros
}

// RawIPToString converts the ip byte arrays to string
func RawIPToString(ip []byte) string {
	return RawIPToAddr(ip).String()
}
