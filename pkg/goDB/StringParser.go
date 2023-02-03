/////////////////////////////////////////////////////////////////////////////////
//
// StringParser.go
//
// Convert string based versions of the goDB keys into goDB internal keys. Useful
// for parsing CSV files.
//
// Written by Lennart Elsen lel@open.ch, May 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
	"errors"
	"strconv"
	"strings"

	"github.com/els0r/goProbe/pkg/goDB/protocols"
)

// StringKeyParser is used for mapping a string to it's goDB key
type StringKeyParser interface {
	ParseKey(element string, key ExtendedKey) error
}

// StringValParser is used for mapping a string to it's goDB value
type StringValParser interface {
	ParseVal(element string, val *Val) error
}

// NewStringKeyParser selects a string parser based on the attribute
func NewStringKeyParser(kind string) StringKeyParser {
	switch kind {
	case "sip":
		return &SipStringParser{}
	case "dip":
		return &DipStringParser{}
	case "dport":
		return &DportStringParser{}
	case "proto":
		return &ProtoStringParser{}
	case "iface":
		return &IfaceStringParser{}
	case "time":
		return &TimeStringParser{}
	}
	return &NOPStringParser{}
}

// NewStringValParser selects a string parser based on a supported goDB counter
func NewStringValParser(kind string) StringValParser {
	switch kind {
	case "packets sent":
		return &PacketsSentStringParser{}
	case "data vol. sent":
		return &BytesSentStringParser{}
	case "packets received":
		return &PacketsRecStringParser{}
	case "data vol. received":
		return &BytesRecStringParser{}
	}
	return &NOPStringParser{}
}

// NOPStringParser doesn't do anything and just lets everything through which
// is not understandable by the other attribute parsers (e.g. the % field or
// any other field not mentioned above)
type NOPStringParser struct{}

// attribute parsers

// SipStringParser parses sip strings
type SipStringParser struct{}

// DipStringParser parses dip strings
type DipStringParser struct{}

// DportStringParser parses dport strings
type DportStringParser struct{}

// ProtoStringParser parses proto strings
type ProtoStringParser struct{}

// extra attributes

// TimeStringParser parses time strings
type TimeStringParser struct{}

// IfaceStringParser parses iface strings
type IfaceStringParser struct{}

// value parsers

// BytesRecStringParser parses bytes received counter strings
type BytesRecStringParser struct{}

// BytesSentStringParser parses bytes sent counter strings
type BytesSentStringParser struct{}

// PacketsRecStringParser parses packets received counter strings
type PacketsRecStringParser struct{}

// PacketsSentStringParser parses packets sent counter strings
type PacketsSentStringParser struct{}

// ParseKey is a no-op
func (n *NOPStringParser) ParseKey(element string, key ExtendedKey) error {
	return nil
}

// ParseVal is a no-op
func (n *NOPStringParser) ParseVal(element string, val *Val) error {
	return nil
}

// ParseKey parses a source IP string and writes it to the source IP key slice
func (s *SipStringParser) ParseKey(element string, key ExtendedKey) error {
	ipBytes, err := IPStringToBytes(element)
	if err != nil {
		return errors.New("Could not parse 'sip' attribute: " + err.Error())
	}
	key.Key().PutSip(ipBytes)
	return nil
}

// ParseKey parses a destination IP string and writes it to the desintation IP key slice
func (d *DipStringParser) ParseKey(element string, key ExtendedKey) error {
	ipBytes, err := IPStringToBytes(element)
	if err != nil {
		return errors.New("Could not parse 'dip' attribute: " + err.Error())
	}
	key.Key().PutDip(ipBytes)
	return nil
}

// ParseKey parses a destination port string and writes it to the desintation port key slice
func (d *DportStringParser) ParseKey(element string, key ExtendedKey) error {
	num, err := strconv.ParseUint(element, 10, 16)
	if err != nil {
		return errors.New("Could not parse 'dport' attribute: " + err.Error())
	}
	key.Key().PutDport([]byte{uint8(num >> 8), uint8(num & 0xff)})
	return nil
}

// ParseKey parses an IP protocol  string and writes it to the protocol key slice
func (p *ProtoStringParser) ParseKey(element string, key ExtendedKey) error {
	var (
		num  uint64
		err  error
		isIn bool
	)

	// first try to parse as number (e.g. 6 or 17)
	if num, err = strconv.ParseUint(element, 10, 8); err != nil {
		// then try to parse as string (e.g. TCP or UDP)
		if num, isIn = protocols.GetIPProtoID(strings.ToLower(element)); !isIn {
			return errors.New("Could not parse 'protocol' attribute: " + err.Error())
		}
	}

	key.Key().PutProto(byte(num & 0xff))
	return nil
}

// ParseKey parses a time string and writes it to the Time key
func (t *TimeStringParser) ParseKey(element string, key ExtendedKey) error {
	// parse into number
	num, err := strconv.ParseInt(element, 10, 64)
	if err != nil {
		return err
	}

	attrIface, hasIface := key.AttrIface()
	if hasIface {
		key = key.Key().Extend(num, attrIface)
	}

	return nil
}

// // ParseKey writes element to the Iface key
func (i *IfaceStringParser) ParseKey(element string, key ExtendedKey) error {

	attrTime, hasTime := key.AttrTime()
	if hasTime {
		key = key.Key().Extend(attrTime.Unix(), element)
	}

	return nil
}

// ParseVal parses a number from a string and writes it to the "Byts Recevied" counter in val
func (b *BytesRecStringParser) ParseVal(element string, val *Val) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.NBytesRcvd = num
	return nil
}

// ParseVal parses a number from a string and writes it to the "Byts Sent" counter in val
func (b *BytesSentStringParser) ParseVal(element string, val *Val) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.NBytesSent = num
	return nil
}

// ParseVal parses a number from a string and writes it to the "Packets Received" counter in val
func (p *PacketsRecStringParser) ParseVal(element string, val *Val) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.NPktsRcvd = num
	return nil
}

// ParseVal parses a number from a string and writes it to the "Packets Sent" counter in val
func (p *PacketsSentStringParser) ParseVal(element string, val *Val) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.NPktsSent = num
	return nil
}
