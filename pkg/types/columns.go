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

// ColumnIndex denotes a static index for one of the supported DB columns
type ColumnIndex int

// IPSizeOf implies that the size of an IP column is variable (hence a negative number)
const IPSizeOf = -1

// Indices for all column types
const (
	// First the attribute columns...
	SIPColIdx, _ ColumnIndex = iota, iota
	DIPColIdx, _
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
	SIPSizeof   int = IPSizeOf
	DIPSizeof   int = IPSizeOf
	ProtoSizeof int = 1
	DportSizeof int = 2
)

// Below enumerate the data type names used across goProbe
const (
	TimeName     = "time"
	HostnameName = "hostname"
	HostIDName   = "hostid"
	IfaceName    = "iface"

	SIPName   = "sip"
	DIPName   = "dip"
	DportName = "dport"
	ProtoName = "proto"

	BytesRcvdName = "bytes_rcvd"
	BytesSentName = "bytes_sent"
	PktsRcvdName  = "pkts_rcvd"
	PktsSentName  = "pkts_sent"
)

// IsCounterCol returns if a column is a counter (and hence does
// not use fixed-width encoding)
func (c ColumnIndex) IsCounterCol() bool {
	return c >= ColIdxAttributeCount && c <= PacketsSentColIdx
}

// ColumnSizeofs returns the data sizes for each column
var ColumnSizeofs = [ColIdxCount]int{
	SIPSizeof, DIPSizeof, ProtoSizeof, DportSizeof,
}

// ColumnFileNames returns the name / title for each column
var ColumnFileNames = [ColIdxCount]string{
	SIPName, DIPName, ProtoName, DportName,
	BytesRcvdName, BytesSentName, PktsRcvdName, PktsSentName,
}

// Column denotes a generic column and enforces the existence of certain methods
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

// SIPAttribute implements the source IP attribute
type SIPAttribute struct {
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

// String returns the string representation of the attribute
func (i ipAttribute) String() string {
	return RawIPToString(i.data[:])
}

// Name returns the attributes name
func (SIPAttribute) Name() string {
	return SIPName
}

func (SIPAttribute) attributeMarker() {}

// DIPAttribute implements the destination IP attribute
type DIPAttribute struct {
	ipAttribute
}

// Name returns the attribute's name
func (DIPAttribute) Name() string {
	return DIPName
}

func (DIPAttribute) attributeMarker() {}

// ProtoAttribute implements the IP protocol attribute
type ProtoAttribute struct {
	data uint8
}

// String returns the string representation of the IP protocol  attribute
func (p ProtoAttribute) String() string {
	return protocols.GetIPProto(int(p.data))
}

// Width returns the amount of bytes the IP protocol attribute takes up on disk
func (ProtoAttribute) Width() Width {
	return ProtoWidth
}

// Name returns the IP protocol attribute's name
func (ProtoAttribute) Name() string {
	return ProtoName
}

// Resolvable returns if the IP protocol attribute is resolvable
func (ProtoAttribute) Resolvable() bool {
	return false
}

func (ProtoAttribute) attributeMarker() {}

// DportAttribute implements the destination port attribute
type DportAttribute struct {
	data []byte
}

// Width returns the amount of bytes the destination port attribute takes up on disk
func (DportAttribute) Width() Width {
	return DPortWidth
}

// String returns the string representation of the destination port attribute
func (d DportAttribute) String() string {
	return fmt.Sprint(d.ToUint16())
}

// Resolvable returns if the destination port is resolvable
func (DportAttribute) Resolvable() bool {
	return false
}

// ToUint16 converts the destination port to a uint16 representation
func (d DportAttribute) ToUint16() uint16 {
	return PortToUint16(d.data)
}

// PortToUint16 converts a uint16 number to a port
func PortToUint16(b []byte) uint16 {
	return binary.BigEndian.Uint16(b[:])
}

// Name returns the destination port attribute name
func (DportAttribute) Name() string {
	return DportName
}

func (DportAttribute) attributeMarker() {}

// NewAttribute returns an attribute for the given name. If no such attribute
// exists, an error is returned.
func NewAttribute(name string) (Attribute, error) {
	// name/alias to attribute matching
	switch name {
	case SIPName, "src":
		return SIPAttribute{}, nil
	case DIPName, "dst":
		return DIPAttribute{}, nil
	case ProtoName, "protocol", "ipproto":
		return ProtoAttribute{}, nil
	case DportName, "port":
		return DportAttribute{}, nil
	default:
		return nil, fmt.Errorf("unknown attribute name: '%s'", name)
	}
}

// AllColumns returns a set of all column names / titles
func AllColumns() []string {
	return []string{
		TimeName, HostnameName, HostIDName, IfaceName, SIPName, DIPName, DportName, ProtoName,
	}
}

const attrSep = ","

// Static shorthands for some column combinations
const (
	TalkConvCompoundQuery    = "talk_conv"
	TalkSrcCompoundQuery     = "talk_src"
	TalkDstCompoundQuery     = "talk_dst"
	AppsPortCompoundQuery    = "apps_port"
	AggTalkPortCompoundQuery = "agg_talk_port"
	RawCompoundQuery         = "raw"
)

// ToAttributeNames translates query abbreviations into the attributes they hold
func ToAttributeNames(queryType string) (attrs []string) {
	// covers the case where aliases and attribute/label names are mixed (e.g. talk_conv,dport)
	qtSplit := strings.Split(queryType, attrSep)
	if len(qtSplit) > 1 {
		for _, attr := range qtSplit {
			attrs = append(attrs, ToAttributeNames(attr)...)
		}
		return attrs
	}

	switch queryType {
	case TalkConvCompoundQuery:
		return []string{SIPName, DIPName}
	case TalkSrcCompoundQuery:
		return []string{SIPName}
	case TalkDstCompoundQuery:
		return []string{DIPName}
	case AppsPortCompoundQuery:
		return []string{DportName, ProtoName}
	case AggTalkPortCompoundQuery:
		return []string{SIPName, DIPName, DportName, ProtoName}
	case RawCompoundQuery:
		return AllColumns()
	}
	// We didn't match any of the preset query types, so we are dealing with
	// a comma separated list of attribute names.
	return strings.Split(queryType, attrSep)
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
		case TimeName:
			selector.Timestamp = true
			continue
		case IfaceName:
			selector.Iface = true
			continue
		case HostnameName:
			selector.Hostname = true
			continue
		case HostIDName:
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
