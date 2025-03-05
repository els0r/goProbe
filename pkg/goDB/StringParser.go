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
	"fmt"
	"strconv"
	"strings"

	"github.com/els0r/goProbe/v4/pkg/goDB/protocols"
	"github.com/els0r/goProbe/v4/pkg/types"
)

// ErrIPVersionMismatch signifies that there is an IPv4 / IPv6 mismatch
var ErrIPVersionMismatch error

// StringKeyParser is used for mapping a string to it's goDB key
type StringKeyParser interface {
	ParseKey(element string, key *types.ExtendedKey) error
}

// StringValParser is used for mapping a string to it's goDB value
type StringValParser interface {
	ParseVal(element string, val *types.Counters) error
}

// NewStringKeyParser selects a string parser based on the attribute
func NewStringKeyParser(kind string) StringKeyParser {
	switch kind {
	case types.SIPName:
		return &SIPStringParser{}
	case types.DIPName:
		return &DIPStringParser{}
	case types.DportName:
		return &DportStringParser{}
	case types.ProtoName:
		return &ProtoStringParser{}
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

// SIPStringParser parses sip strings
type SIPStringParser struct{}

// DIPStringParser parses dip strings
type DIPStringParser struct{}

// DportStringParser parses dport strings
type DportStringParser struct{}

// ProtoStringParser parses proto strings
type ProtoStringParser struct{}

// extra attributes

// TimeStringParser parses time strings
type TimeStringParser struct{}

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
func (n *NOPStringParser) ParseKey(_ string, _ *types.ExtendedKey) error {
	return nil
}

// ParseVal is a no-op
func (n *NOPStringParser) ParseVal(_ string, _ *types.Counters) error {
	return nil
}

// ParseKey parses a source IP string and writes it to the source IP key slice
func (s *SIPStringParser) ParseKey(element string, key *types.ExtendedKey) error {
	ipBytes, _, err := types.IPStringToBytes(element)
	if err != nil {
		return fmt.Errorf("could not parse 'sip' attribute: %w", err)
	}
	if (len(ipBytes) == 4) != key.Key().IsIPv4() {
		return ErrIPVersionMismatch
	}

	key.Key().PutSIP(ipBytes)
	return nil
}

// ParseKey parses a destination IP string and writes it to the desintation IP key slice
func (d *DIPStringParser) ParseKey(element string, key *types.ExtendedKey) error {
	ipBytes, _, err := types.IPStringToBytes(element)
	if err != nil {
		return fmt.Errorf("could not parse 'dip' attribute: %w", err)
	}
	if (len(ipBytes) == 4) != key.Key().IsIPv4() {
		return ErrIPVersionMismatch
	}

	key.Key().PutDIP(ipBytes)
	return nil
}

// ParseKey parses a destination port string and writes it to the desintation port key slice
func (d *DportStringParser) ParseKey(element string, key *types.ExtendedKey) error {
	num, err := strconv.ParseUint(element, 10, 16)
	if err != nil {
		return fmt.Errorf("could not parse 'dport' attribute: %w", err)
	}
	key.Key().PutDport([]byte{uint8(num >> 8), uint8(num & 0xff)})
	return nil
}

// ParseKey parses an IP protocol  string and writes it to the protocol key slice
func (p *ProtoStringParser) ParseKey(element string, key *types.ExtendedKey) error {
	var (
		num  uint64
		err  error
		isIn bool
	)

	// first try to parse as number (e.g. 6 or 17)
	if num, err = strconv.ParseUint(element, 10, 8); err != nil {
		// then try to parse as string (e.g. TCP or UDP)
		if num, isIn = protocols.GetIPProtoID(strings.ToLower(element)); !isIn {
			return fmt.Errorf("could not parse 'protocol' attribute: %w", err)
		}
	}

	key.Key().PutProto(byte(num & 0xff))
	return nil
}

// ParseKey parses a time string and writes it to the Time key
func (t *TimeStringParser) ParseKey(element string, key *types.ExtendedKey) error {
	// parse into number
	num, err := strconv.ParseInt(element, 10, 64)
	if err != nil {
		return err
	}

	*key = key.Key().Extend(num)

	return nil
}

// ParseVal parses a number from a string and writes it to the "Byts Recevied" counter in val
func (b *BytesRecStringParser) ParseVal(element string, val *types.Counters) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.BytesRcvd = num
	return nil
}

// ParseVal parses a number from a string and writes it to the "Byts Sent" counter in val
func (b *BytesSentStringParser) ParseVal(element string, val *types.Counters) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.BytesSent = num
	return nil
}

// ParseVal parses a number from a string and writes it to the "Packets Received" counter in val
func (p *PacketsRecStringParser) ParseVal(element string, val *types.Counters) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.PacketsRcvd = num
	return nil
}

// ParseVal parses a number from a string and writes it to the "Packets Sent" counter in val
func (p *PacketsSentStringParser) ParseVal(element string, val *types.Counters) error {
	// parse into number
	num, err := strconv.ParseUint(element, 10, 64)
	if err != nil {
		return err
	}

	val.PacketsSent = num
	return nil
}
