/////////////////////////////////////////////////////////////////////////////////
//
// node_test.go
//
//
// Written by Lorenz Breidenbach lob@open.ch, September 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package node

import (
	"errors"
	"testing"
)

var parseConditionalTests = []struct {
	inTokens  []string
	astString string
	success   bool
}{
	{[]string{"host", "!=", "192.168.178.1", "|", "(", "host", "=", "192.168.178.1", ")", ")"}, "", false},
	{[]string{"host", "="}, "", false},
	{[]string{"sip", "=", "192.168.1.1", "|", "(", "host"}, "", false},
	{[]string{"sip", "=", "192.168.1.1", "/", "(", "sip"}, "", false},
	{[]string{"(", "sip", "=", "192.168.1.1", ")"}, "", true},
	{[]string{"sip", "=", "192.168.1.1", ")"}, "", false},
	{[]string{"sip", "$", "192.168.1.1"}, "", false},
	{[]string{"(", "sip", "=", "192.168.1.1"}, "", false},
	{[]string{"sip", "&", "192.168.1.1"}, "", false},
	{[]string{"sip", "=", "192.168.1.1"},
		"sip = 192.168.1.1",
		true},
	{[]string{"sip", "=", "www.example.com", "|", "dip", "=", "dip.example.com"},
		"(sip = www.example.com) | (dip = dip.example.com)",
		true},
	{[]string{"!", "sip", "=", "192.168.1.2", "|", "!", "dip", "=", "192.168.1.1", "|", "dport", "!=", "80"},
		"(!(sip = 192.168.1.2) | (!(dip = 192.168.1.1) | dport != 80))",
		true},
	{[]string{"sip", "=", "192.168.1.1", "|", "sip", "=", "192.168.1.2", "|", "sip", "=", "192.168.1.3", "|", "sip", "=", "192.168.1.4"},
		"(sip = 192.168.1.1 | (sip = 192.168.1.2 | (sip = 192.168.1.3 | sip = 192.168.1.4)))",
		true},
	{[]string{"dir", "=", "in"},
		"dir = in",
		true},
	{[]string{"direction", "=", "in"},
		"direction = in",
		true},
	{[]string{"directio", "=", "in"},
		"direction = in",
		false},
}

func TestParseConditional(t *testing.T) {
	for _, test := range parseConditionalTests {
		ast, err := parseConditional(test.inTokens)
		if (err == nil) != test.success {
			t.Log("ast", ast)
			t.Log("err", err)
			t.Fatalf("Test: %v, Expected output: %v. Actual output: %v", test.inTokens, test.success, err == nil)
		}
		if test.success {
			t.Log("AST:", ast)
		} else {
			t.Logf("ERROR:\n%s\n", err)
		}
	}
}

func TestParseConditionalEmpty(t *testing.T) {
	ast, err := parseConditional(nil)
	if ast != nil || err == nil || !errors.Is(err, errEmptyConditional) {
		t.Fatalf("TestParseConditionalEmpty expected: nil, nil Got: %v, %v", ast, err)
	}

	ast, err = parseConditional([]string{})
	if ast != nil || err == nil || !errors.Is(err, errEmptyConditional) {
		t.Fatalf("TestParseConditionalEmpty expected: nil, nil Got: %v, %v", ast, err)
	}
}
