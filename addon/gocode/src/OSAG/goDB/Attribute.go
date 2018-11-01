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
)

// Interface for attributes
// This interface is not meant to be implemented by structs
// outside this package
type Attribute interface {
	Name() string
	// Some attributes use more than a single output column.
	ExtraColumns() []string
	// ExtractStrings() extracts a list of records representing the
	// attribute from a given key.
	// You may assume that the length of the returned list is always
	// the length of ExtraColumns() + 1.
	ExtractStrings(key *ExtraKey) []string
	// Ensures that this interface cannot be implemented outside this
	// package.
	attributeMarker()
}

type SipAttribute struct{}

func (_ SipAttribute) Name() string {
	return "sip"
}
func (_ SipAttribute) ExtraColumns() []string {
	return nil
}
func (_ SipAttribute) ExtractStrings(key *ExtraKey) []string {
	return []string{rawIpToString(key.Sip[:])}
}
func (_ SipAttribute) attributeMarker() {}

type DipAttribute struct{}

func (_ DipAttribute) Name() string {
	return "dip"
}
func (_ DipAttribute) ExtraColumns() []string {
	return nil
}
func (_ DipAttribute) ExtractStrings(key *ExtraKey) []string {
	return []string{rawIpToString(key.Dip[:])}
}
func (_ DipAttribute) attributeMarker() {}

type ProtoAttribute struct{}

func (_ ProtoAttribute) Name() string {
	return "proto"
}
func (_ ProtoAttribute) ExtraColumns() []string {
	return nil
}
func (_ ProtoAttribute) ExtractStrings(key *ExtraKey) []string {
	return []string{GetIPProto(int(key.Protocol))}
}
func (_ ProtoAttribute) attributeMarker() {}

type DportAttribute struct{}

func (_ DportAttribute) Name() string {
	return "dport"
}
func (_ DportAttribute) ExtraColumns() []string {
	return nil
}
func (_ DportAttribute) ExtractStrings(key *ExtraKey) []string {
	return []string{strconv.Itoa(int(uint16(key.Dport[0])<<8 | uint16(key.Dport[1])))}
}
func (_ DportAttribute) attributeMarker() {}

// Returns an Attribute for the given name. If no such attribute
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

// Parses the given query type into a list of attributes.
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

// Find out if any of the attributes are usable for a reverse DNS lookup
// (e.g. check for IP attributes)
func HasDNSAttributes(attributes []Attribute) bool {
	for _, attr := range attributes {
		if attr.Name() == "sip" || attr.Name() == "dip" {
			return true
		}
	}
	return false
}
