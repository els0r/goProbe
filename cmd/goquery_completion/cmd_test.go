/////////////////////////////////////////////////////////////////////////////////
//
// cmd_test.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import (
	"reflect"
	"testing"
)

var unescapeTests = []struct {
	in    string
	out   []string
	weird bool
}{
	{"", []string{""}, false},
	{" ", []string{""}, false},
	{"  ", []string{""}, false},
	{"a", []string{"a"}, false},
	{"a  ", []string{"a", ""}, false},
	{"  a  ", []string{"a", ""}, false},
	{"  'a'  ", []string{"a", ""}, false},
	{`\  "a"  `, []string{" ", "a", ""}, false},
	{`\ "\ \\\"\n"`, []string{" ", `\ \"\n`, ""}, true},
	{` "a" \  `, []string{"a", " ", ""}, false},
	{` "a"\ `, []string{"a", " "}, false},
	{`\  'a'  `, []string{" ", "a", ""}, false},
	{` 'a' \  `, []string{"a", " ", ""}, false},
	{` 'a'\ `, []string{"a", " "}, false},
	{`"hello""world`, []string{"hello", "world"}, false},
	{`"hello""world"`, []string{"hello", "world", ""}, true},
	{`"hello"'world'`, []string{"hello", "world", ""}, true},
	{`"hello"'world"`, []string{"hello", "world\""}, false},
	{`"world'`, []string{"world'"}, false},
	{`''`, []string{"", ""}, true},
}

func TestBashUnescape(t *testing.T) {
	for _, test := range unescapeTests {
		out, weird := bashUnescape(test.in)
		if !reflect.DeepEqual(test.out, out) || test.weird != weird {
			t.Fatalf("Expected (%#v, %v), got (%#v, %v)", test.out, test.weird, out, weird)
		}
	}

}
