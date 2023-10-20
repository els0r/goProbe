/////////////////////////////////////////////////////////////////////////////////
//
// Attribute_test.go
//
// Written by Lorenz Breidenbach lob@open.ch, November 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package types

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	SIP      = [16]byte{0xA1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	DIP      = [16]byte{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}
	Dport    = []byte{0xCB, 0xF1}
	Protocol = uint8(6)
	Time     = 0
)

var tests = []struct {
	Attribute        Attribute
	Name             string
	ExtractedStrings string
}{
	{SIPAttribute{ipAttribute{data: SIP[:]}}, "sip", "a102:304:506:708:90a:b0c:d0e:f10"},
	{DIPAttribute{ipAttribute{data: DIP[:]}}, "dip", "301:401:509:206:503:508:907:903"},
	{DportAttribute{Dport}, "dport", "52209"},
	{ProtoAttribute{Protocol}, "proto", "TCP"},
}

func TestAttributes(t *testing.T) {
	for _, test := range tests {
		if test.Attribute.Name() != test.Name {
			t.Fatalf("wrong name")
		}
		es := test.Attribute.String()
		if !reflect.DeepEqual(es, test.ExtractedStrings) {
			t.Fatalf("%s: expected: %s got: %s", test.Attribute.Name(), test.ExtractedStrings, es)
		}
	}
}

func TestNewAttribute(t *testing.T) {
	for _, name := range []string{"sip", "dip", "dport", "proto"} {
		attrib, err := NewAttribute(name)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if name != attrib.Name() {
			t.Fatalf("Wrong attribute")
		}
	}

	attrib, err := NewAttribute("src")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if attrib.Name() != "sip" {
		t.Fatalf("Wrong attribute")
	}

	attrib, err = NewAttribute("dst")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if attrib.Name() != "dip" {
		t.Fatalf("Wrong attribute")
	}

	_, err = NewAttribute("time")
	if err == nil {
		t.Fatalf("Expected error")
	}
}

var parseQueryTypeTests = []struct {
	InQueryType     string
	OutAttributes   []Attribute
	OutHasAttrTime  bool
	OutHasAttrIface bool
}{
	{"sip", []Attribute{SIPAttribute{}}, false, false},
	{"src", []Attribute{SIPAttribute{}}, false, false},
	{"dst", []Attribute{DIPAttribute{}}, false, false},
	{"talk_src", []Attribute{SIPAttribute{}}, false, false},
	{"sip,dip,dip,sip,dport", []Attribute{SIPAttribute{}, DIPAttribute{}, DportAttribute{}}, false, false},
	{"sip,dip,dip,iface,sip,dport", []Attribute{SIPAttribute{}, DIPAttribute{}, DportAttribute{}}, false, true},
	{"sip,dip,dst,src,dport", []Attribute{SIPAttribute{}, DIPAttribute{}, DportAttribute{}}, false, false},
	{"src,dst,dip,sip,dport", []Attribute{SIPAttribute{}, DIPAttribute{}, DportAttribute{}}, false, false},
	{"sip,dip,dip,sip,dport,talk_src", []Attribute{SIPAttribute{}, DIPAttribute{}, DportAttribute{}}, false, false},
	{"sip,dip,time,dip,sip,dport", []Attribute{SIPAttribute{}, DIPAttribute{}, DportAttribute{}}, true, false},
	{"talk_src,dip", []Attribute{SIPAttribute{}, DIPAttribute{}}, false, false},
	{"talk_src,src", []Attribute{SIPAttribute{}}, false, false},
	{"raw", []Attribute{SIPAttribute{}, DIPAttribute{}, DportAttribute{}, ProtoAttribute{}}, true, true},
}

func TestParseQueryType(t *testing.T) {
	for i, test := range parseQueryTypeTests {
		test := test
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			attributes, selector, err := ParseQueryType(test.InQueryType)
			require.Nilf(t, err, "Unexpectedly failed on input %v", test.InQueryType)
			require.Equal(t, test.OutHasAttrIface, selector.Iface)
			require.Equal(t, test.OutHasAttrTime, selector.Timestamp)
			require.Equal(t, test.OutAttributes, attributes)
		})
	}
}

func TestParseQueryError(t *testing.T) {
	var tests = []struct {
		name        string
		query       string
		expectedErr *ParseError
	}{
		{"all valid", "sip,dip,dip,sip,dport,talk_src", nil},
		{"empty query", "",
			NewParseError([]string{""}, 0, ",", errorUnknownAttribute.Error())},
		{"incorrect first", "sipl,talk_src",
			NewParseError([]string{"sipl", "sip"}, 0, ",", errorUnknownAttribute.Error())},
		{"incorrect middle", "sip,dipl,talk_src",
			NewParseError([]string{"sip", "dipl", "sip"}, 1, ",", errorUnknownAttribute.Error())},
		{"incorrect end", "sip,dip,talk_src,d",
			NewParseError([]string{"sip", "dip", "sip", "d"}, 3, ",", errorUnknownAttribute.Error())},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_, _, err := ParseQueryType(test.query)
			t.Logf("error:\n%v", err)

			if test.expectedErr == nil {
				require.Nil(t, err)
				return
			}

			require.Equal(t, test.expectedErr, err)
		})
	}
}
