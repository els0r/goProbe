/////////////////////////////////////////////////////////////////////////////////
//
// Attribute.go
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, November 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package types

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/els0r/goProbe/pkg/goDB/protocols"
)

type ColumnIndex int

const IPSizeOf = -1

// Indices for all column types
const (
	// First the attribute columns...
	SipColIdx, _ ColumnIndex = iota, iota
	DipColIdx, _
	ProtoColIdx, _
	DportColIdx, _

	// ... and then the columns we aggregate
	BytesRcvdColIdx, ColIdxAttributeCount
	BytesSentColIdx, _
	PacketsRcvdColIdx, _
	PacketsSentColIdx, _
	ColIdxCount, _
)

// Sizeof (entry) for all column types
const (
	SipSizeof   int = IPSizeOf
	DipSizeof   int = IPSizeOf
	ProtoSizeof int = 1
	DportSizeof int = 2
)

// IsCounterCol returns if a column is a counter (and hence does
// not use fixed-width encoding)
func (c ColumnIndex) IsCounterCol() bool {
	return c >= ColIdxAttributeCount && c <= PacketsSentColIdx
}

var ColumnSizeofs = [ColIdxCount]int{
	SipSizeof, DipSizeof, ProtoSizeof, DportSizeof,
}

var ColumnFileNames = [ColIdxCount]string{
	"sip", "dip", "proto", "dport",
	"bytes_rcvd", "bytes_sent", "pkts_rcvd", "pkts_sent"}

type Column interface {
	Name() string
	Width() Width
}

// Attribute interface. It is not meant to be implemented by structs
// outside this package
type Attribute interface {
	// the attribute needs to be able to represent itself as a String
	fmt.Stringer

	// an Attribute is a columnn
	Column

	// Resolvable defines whether the attribute can be resolved via a DNS
	// reverse lookup
	Resolvable() bool

	// Ensures that this interface cannot be implemented outside this
	// package.
	attributeMarker()
}

type ipAttribute struct {
	data []byte
}

// SipAttribute implements the source IP attribute
type SipAttribute struct {
	ipAttribute
}

// Width returns the amount of bytes the IP attribute takes up on disk
func (i ipAttribute) Width() Width {
	return len(i.data)
}

// Resolvable defines whether the attribute can be resolved via a DNS
func (ipAttribute) Resolvable() bool {
	return true
}

func (i ipAttribute) String() string {
	return RawIPToString(i.data[:])
}

// Name returns the attributes name
func (SipAttribute) Name() string {
	return "sip"
}

func (SipAttribute) attributeMarker() {}

// DipAttribute implements the destination IP attribute
type DipAttribute struct {
	ipAttribute
}

// Name returns the attribute's name
func (DipAttribute) Name() string {
	return "dip"
}

func (DipAttribute) attributeMarker() {}

// ProtoAttribute implements the IP protocol attribute
type ProtoAttribute struct {
	data uint8
}

func (p ProtoAttribute) String() string {
	return protocols.GetIPProto(int(p.data))
}

func (ProtoAttribute) Width() Width {
	return ProtoWidth
}

// Name returns the attribute's name
func (ProtoAttribute) Name() string {
	return "proto"
}

func (ProtoAttribute) Resolvable() bool {
	return false
}

func (ProtoAttribute) attributeMarker() {}

// DportAttribute implements the destination port attribute
type DportAttribute struct {
	data []byte
}

func (DportAttribute) Width() Width {
	return DPortWidth
}

func (d DportAttribute) String() string {
	return fmt.Sprint(d.ToUint16())
}

func (DportAttribute) Resolvable() bool {
	return false
}

func (d DportAttribute) ToUint16() uint16 {
	return PortToUint16(d.data)
}

func PortToUint16(b []byte) uint16 {
	return binary.BigEndian.Uint16(b[:])
}

// Name returns the attribute's name
func (DportAttribute) Name() string {
	return "dport"
}

func (DportAttribute) attributeMarker() {}

// NewAttribute returns an Attribute for the given name. If no such attribute
// exists, an error is returned.
func NewAttribute(name string) (Attribute, error) {
	switch name {
	case "sip", "src": // src is an alias for sip
		return SipAttribute{}, nil
	case "dip", "dst": // dst is an alias for dip
		return DipAttribute{}, nil
	case "proto", "protocol", "ipproto": // ipproto/protocol is an alias for proto
		return ProtoAttribute{}, nil
	case "dport", "port": // port is an alias for dport
		return DportAttribute{}, nil
	default:
		return nil, fmt.Errorf("Unknown attribute name: '%s'", name)
	}
}

func AllColumns() []string {
	return []string{"time", "hostname", "hostid", "iface", "sip", "dip", "dport", "proto"}
}

// ToAttribueNames translates query abbreviations into the attributes they hold
func ToAttributeNames(queryType string) []string {
	switch queryType {
	case "talk_conv":
		return []string{"sip", "dip"}
	case "talk_src":
		return []string{"sip"}
	case "talk_dst":
		return []string{"dip"}
	case "apps_port":
		return []string{"dport", "proto"}
	case "agg_talk_port":
		return []string{"sip", "dip", "dport", "proto"}
	case "raw":
		return AllColumns()
	}
	// We didn't match any of the preset query types, so we are dealing with
	// a comma separated list of attribute names.
	return strings.Split(queryType, ",")
}

// ParseQueryType parses the given query type into a list of attributes.
// The returned list is guaranteed to have no duplicates.
// A valid query type can either be a comma-separated list of
// attribute names (e.g. "sip,dip,dport") or something like
// "talk_conv".
// The return variable hasAttrTime indicates whether the special
// time attribute is present. (time is never a part of the returned
// attribute list.) The time attribute is present for the query type
// 'raw', or if it is explicitly mentioned in a list of attribute
// names.
func ParseQueryType(queryType string) (attributes []Attribute, selector LabelSelector, err error) {
	attributeNames := ToAttributeNames(queryType)
	attributeSet := make(map[string]struct{})
	for _, attributeName := range attributeNames {
		switch attributeName {
		case "time":
			selector.Timestamp = true
			continue
		case "iface":
			selector.Iface = true
			continue
		case "hostname":
			selector.Hostname = true
			continue
		case "hostid":
			selector.HostID = true
			continue
		}

		attribute, err := NewAttribute(attributeName)
		if err != nil {
			return nil, LabelSelector{}, err
		}
		if _, exists := attributeSet[attribute.Name()]; !exists {
			attributeSet[attribute.Name()] = struct{}{}
			attributes = append(attributes, attribute)
		}
	}
	return
}

// HasDNSAttributes finds out if any of the attributes are usable for a reverse DNS lookup
// (e.g. check for IP attributes)
func HasDNSAttributes(attributes []Attribute) bool {
	for _, attr := range attributes {
		if attr.Resolvable() {
			return true
		}
	}
	return false
}
