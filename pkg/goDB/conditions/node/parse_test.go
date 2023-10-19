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

	"github.com/stretchr/testify/require"
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

func TestParseError(t *testing.T) {
	var tests = []struct {
		name        string
		tokens      []string
		expectedErr *ParseError
	}{
		{"missing comparison operator",
			[]string{"sip", "=", "192.168.1.1", "|", "(", "host"},
			&ParseError{
				Pos:         6,
				Description: `Expected comparison operator`,
			},
		},
		{"incorrect attribute beginning",
			[]string{"sipl", "=", "192.168.1.1"},
			&ParseError{
				Pos:         0,
				Description: `Expected attribute`,
			},
		},
		{"incorrect attribute middle",
			[]string{"sip", "=", "192.168.1.1", "&", "dipl", "=", "192.168.1.2"},
			&ParseError{
				Pos:         4,
				Description: `Expected attribute`,
			},
		},
	}
	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			// test parse error
			_, err := parseConditional(test.tokens)
			if test.expectedErr == nil {
				require.Nil(t, err)
				return
			}
			t.Logf("error:\n%v", err)

			test.expectedErr.Tokens = test.tokens
			require.Equal(t, test.expectedErr, err)
		})
	}
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
