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
)

type StringKeyParser interface {
	ParseKey(element string, key *ExtraKey) error
}

type StringValParser interface {
	ParseVal(element string, val *Val) error
}

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

type NOPStringParser struct{}

// attribute parsers
type SipStringParser struct{}
type DipStringParser struct{}
type DportStringParser struct{}
type ProtoStringParser struct{}

// extra attributes
type TimeStringParser struct{}
type IfaceStringParser struct{}

// value parsers
type BytesRecStringParser struct{}
type BytesSentStringParser struct{}
type PacketsRecStringParser struct{}
type PacketsSentStringParser struct{}

// The NOP parser doesn't do anything and just lets everything through which
// is not understandable by the other attribute parsers (e.g. the % field or
// any other field not mentioned above)
func (n *NOPStringParser) ParseKey(element string, key *ExtraKey) error {
	return nil
}

func (n *NOPStringParser) ParseVal(element string, val *Val) error {
	return nil
}

func (s *SipStringParser) ParseKey(element string, key *ExtraKey) error {
	ipBytes, err := IPStringToBytes(element)
	if err != nil {
		return errors.New("Could not parse 'sip' attribute: " + err.Error())
	}
	copy(key.Sip[:], ipBytes[:])
	return nil
}
func (d *DipStringParser) ParseKey(element string, key *ExtraKey) error {
	ipBytes, err := IPStringToBytes(element)
	if err != nil {
		return errors.New("Could not parse 'dip' attribute: " + err.Error())
	}
	copy(key.Dip[:], ipBytes[:])
	return nil
}
func (d *DportStringParser) ParseKey(element string, key *ExtraKey) error {
	num, err := strconv.ParseUint(element, 10, 16)
	if err != nil {
		return errors.New("Could not parse 'dport' attribute: " + err.Error())
	}
	copy(key.Dport[:], []byte{uint8(num >> 8), uint8(num & 0xff)})
	return nil
}
func (p *ProtoStringParser) ParseKey(element string, key *ExtraKey) error {
	var (
		num  uint64
		err  error
		isIn bool
	)

	// first try to parse as number (e.g. 6 or 17)
	if num, err = strconv.ParseUint(element, 10, 8); err != nil {
		// then try to parse as string (e.g. TCP or UDP)
		if num, isIn = GetIPProtoID(strings.ToLower(element)); !isIn {
			return errors.New("Could not parse 'protocol' attribute: " + err.Error())
		}
	}

	key.Protocol = byte(num & 0xff)
	return nil
}
func (t *TimeStringParser) ParseKey(element string, key *ExtraKey) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}
	key.Time = int64(num)
	return nil
}
func (i *IfaceStringParser) ParseKey(element string, key *ExtraKey) error {
	key.Iface = element
	return nil
}
func (b *BytesRecStringParser) ParseVal(element string, val *Val) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.NBytesRcvd = num
	return nil
}

func (b *BytesSentStringParser) ParseVal(element string, val *Val) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.NBytesSent = num
	return nil
}

func (p *PacketsRecStringParser) ParseVal(element string, val *Val) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.NPktsRcvd = num
	return nil
}

func (p *PacketsSentStringParser) ParseVal(element string, val *Val) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.NPktsSent = num
	return nil
}
