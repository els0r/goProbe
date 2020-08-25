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

package goDB

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/els0r/goProbe/pkg/goDB/protocols"
)

// Attribute interface. It is not meant to be implemented by structs
// outside this package
type Attribute interface {
	Name() string

	// ExtractStrings() extracts a list of records representing the
	// attribute from a given key.
	ExtractStrings(key *ExtraKey) []string

	// Ensures that this interface cannot be implemented outside this
	// package.
	attributeMarker()
}

// SipAttribute implements the source IP attribute
type SipAttribute struct{}

// Name returns the attributes name
func (SipAttribute) Name() string {
	return "sip"
}

// ExtractStrings converts the sip byte slice into a human-readable IP address
func (SipAttribute) ExtractStrings(key *ExtraKey) []string {
	return []string{RawIPToString(key.Sip[:])}
}

func (SipAttribute) attributeMarker() {}

// DipAttribute implements the destination IP attribute
type DipAttribute struct{}

// Name returns the attribute's name
func (DipAttribute) Name() string {
	return "dip"
}

// ExtractStrings converts the dip byte slice into a human-readable IP address
func (DipAttribute) ExtractStrings(key *ExtraKey) []string {
	return []string{RawIPToString(key.Dip[:])}
}
func (DipAttribute) attributeMarker() {}

// ProtoAttribute implements the IP protocol attribute
type ProtoAttribute struct{}

// Name returns the attribute's name
func (ProtoAttribute) Name() string {
	return "proto"
}

// ExtractStrings converts the numeric IP protocol into a human-readable name (e.g. "UDP")
func (ProtoAttribute) ExtractStrings(key *ExtraKey) []string {
	return []string{protocols.GetIPProto(int(key.Protocol))}
}

func (ProtoAttribute) attributeMarker() {}

// DportAttribute implements the destination port attribute
type DportAttribute struct{}

// Name returns the attribute's name
func (DportAttribute) Name() string {
	return "dport"
}

// ExtractStrings converts the dport byte slice into a numeric port number (e.g. 443)
func (DportAttribute) ExtractStrings(key *ExtraKey) []string {
	return []string{strconv.Itoa(int(uint16(key.Dport[0])<<8 | uint16(key.Dport[1])))}
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
	case "proto":
		return ProtoAttribute{}, nil
	case "dport":
		return DportAttribute{}, nil
	default:
		return nil, fmt.Errorf("Unknown attribute name: '%s'", name)
	}
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
func ParseQueryType(queryType string) (attributes []Attribute, hasAttrTime, hasAttrIface bool, err error) {
	switch queryType {
	case "talk_conv":
		return []Attribute{SipAttribute{}, DipAttribute{}}, false, false, nil
	case "talk_src":
		return []Attribute{SipAttribute{}}, false, false, nil
	case "talk_dst":
		return []Attribute{DipAttribute{}}, false, false, nil
	case "apps_port":
		return []Attribute{DportAttribute{}, ProtoAttribute{}}, false, false, nil
	case "agg_talk_port":
		return []Attribute{SipAttribute{}, DipAttribute{}, DportAttribute{}, ProtoAttribute{}}, false, false, nil
	case "raw":
		return []Attribute{SipAttribute{}, DipAttribute{}, DportAttribute{}, ProtoAttribute{}}, true, true, nil
	}
	// We didn't match any of the preset query types, so we are dealing with
	// a comma separated list of attribute names.
	attributeNames := strings.Split(queryType, ",")
	attributeSet := make(map[string]struct{})
	for _, attributeName := range attributeNames {
		switch attributeName {
		case "time":
			hasAttrTime = true
			continue
		case "iface":
			hasAttrIface = true
			continue
		}

		attribute, err := NewAttribute(attributeName)
		if err != nil {
			return nil, false, false, err
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
		if attr.Name() == "sip" || attr.Name() == "dip" {
			return true
		}
	}
	return false
}
