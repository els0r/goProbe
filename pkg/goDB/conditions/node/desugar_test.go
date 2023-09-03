/////////////////////////////////////////////////////////////////////////////////
//
// desugar_test.go
//
// Written by Lorenz Breidenbach lob@open.ch, January 2016
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package node

import (
	"errors"
	"testing"
)

var desugarTests = []struct {
	inTokens []string
	output   string // desugared ouput
	success  bool
}{
	{
		[]string{"host", "!=", "192.168.178.1", "|", "(", "host", "=", "192.168.178.1", ")"},
		"(!((sip = 192.168.178.1 | dip = 192.168.178.1)) | (sip = 192.168.178.1 | dip = 192.168.178.1))",
		true,
	},
	{
		[]string{"net", "!=", "192.168.178.1/24", "|", "(", "net", "=", "192.168.178.1/16", ")"},
		"(!((snet = 192.168.178.1/24 | dnet = 192.168.178.1/24)) | (snet = 192.168.178.1/16 | dnet = 192.168.178.1/16))",
		true,
	},
	{
		[]string{"!", "(", "src", "=", "192.168.178.1", "&", "dst", "!=", "1.2.3.4", ")"},
		"!((sip = 192.168.178.1 & dip != 1.2.3.4))",
		true,
	},
	{
		[]string{"host", "<", "192.168.178.1/24"},
		"",
		false,
	},
	{
		[]string{"net", ">=", "192.168.178.1/24", "|", "(", "net", "=", "192.168.178.1/16", ")"},
		"",
		false,
	},
}

func TestDesugar(t *testing.T) {
	for _, test := range desugarTests {
		node, err := parseConditional(test.inTokens)
		if err != nil && !errors.Is(err, errEmptyConditional) {
			t.Fatalf("Parsing %v unexpectly failed. Error:\n%v", test.inTokens, err)
		}

		desugaredNode, err := desugar(node)
		if !test.success {
			if err == nil {
				t.Fatalf("Expected to fail on input %v but didn't.",
					test.inTokens)
			}
		} else {
			if err != nil {
				t.Fatalf("Unexpectedly failed on input %v. The error is: %s",
					test.inTokens, err)
			}
			if desugaredNode.String() != test.output {
				t.Fatalf("Expected output: %s. Actual output: %s",
					test.output, desugaredNode)
			}
		}
	}
}
