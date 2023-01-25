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
