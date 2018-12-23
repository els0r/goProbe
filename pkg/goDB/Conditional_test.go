/////////////////////////////////////////////////////////////////////////////////
//
// Conditional_test.go
//
//
// Written by Lorenz Breidenbach lob@open.ch, September 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
	"fmt"
	"testing"
)

var negationNormalFormTests = []struct {
	inTokens []string
	output   string
}{
	//No change
	{[]string{"sip", "!=", "127.0.0.1"}, "sip != 127.0.0.1"},
	{[]string{"sip", "=", "127.0.0.1"}, "sip = 127.0.0.1"},
	{[]string{"sip", ">=", "127.0.0.1"}, "sip >= 127.0.0.1"},
	{[]string{"sip", "<=", "127.0.0.1"}, "sip <= 127.0.0.1"},
	{[]string{"sip", "<", "127.0.0.1"}, "sip < 127.0.0.1"},
	{[]string{"sip", ">", "127.0.0.1"}, "sip > 127.0.0.1"},
	//Flip comparison op
	{[]string{"!", "sip", "!=", "127.0.0.1"}, "sip = 127.0.0.1"},
	{[]string{"!", "sip", "=", "127.0.0.1"}, "sip != 127.0.0.1"},
	{[]string{"!", "sip", ">=", "127.0.0.1"}, "sip < 127.0.0.1"},
	{[]string{"!", "sip", "<=", "127.0.0.1"}, "sip > 127.0.0.1"},
	{[]string{"!", "sip", "<", "127.0.0.1"}, "sip >= 127.0.0.1"},
	{[]string{"!", "sip", ">", "127.0.0.1"}, "sip <= 127.0.0.1"},
	//Double negation
	{[]string{"!", "(", "!", "sip", "!=", "127.0.0.1", ")"}, "sip != 127.0.0.1"},
	//Logical connectives
	{[]string{"sip", "!=", "127.0.0.1", "&", "sip", "!=", "192.168.0.1"}, "(sip != 127.0.0.1 & sip != 192.168.0.1)"},
	{[]string{"sip", "!=", "127.0.0.1", "|", "sip", "!=", "192.168.0.1"}, "(sip != 127.0.0.1 | sip != 192.168.0.1)"},
	//Nested formula
	{[]string{"!", "(", "!", "sip", "!=", "127.0.0.1", "|", "dport", "<", "80", ")"}, "(sip != 127.0.0.1 & dport >= 80)"},
}

func TestNegationNormalForm(t *testing.T) {
	for _, test := range negationNormalFormTests {
		node, err := parseConditional(test.inTokens)
		if err != nil {
			t.Fatalf("Parsing %v unexpectly failed. Error:\n%v", test.inTokens, err)
		}
		nnfNode := negationNormalForm(node)
		if nnfNode.String() != test.output {
			t.Fatalf("Expected output: %v Actual output: %v", test.output, nnfNode)
		}
	}
}

var listToTreeTests = []struct {
	and     bool
	inNodes []Node
	output  string
}{
	{true, []Node{newConditionNode("dport", "=", "10")}, "dport = 10"},
	{true, []Node{newConditionNode("dport", "=", "10"), newConditionNode("dport", "=", "11")}, "(dport = 10 & dport = 11)"},
	{true, []Node{newConditionNode("dport", "=", "10"), newConditionNode("dport", "=", "11"), newConditionNode("dport", "=", "12")}, "(dport = 10 & (dport = 11 & dport = 12))"},
	{false, []Node{newConditionNode("dport", "=", "10")}, "dport = 10"},
	{false, []Node{newConditionNode("dport", "=", "10"), newConditionNode("dport", "=", "11")}, "(dport = 10 | dport = 11)"},
	{false, []Node{newConditionNode("dport", "=", "10"), newConditionNode("dport", "=", "11"), newConditionNode("dport", "=", "12")}, "(dport = 10 | (dport = 11 | dport = 12))"},
}

func TestListToTree(t *testing.T) {
	var checkNoPointer func(Node) bool
	checkNoPointer = func(node Node) bool {
		switch node := node.(type) {
		case *andNode:
			return false
		case *orNode:
			return false
		case *notNode:
			return false
		case *conditionNode:
			return false
		case andNode:
			return checkNoPointer(node.left) && checkNoPointer(node.right)
		case orNode:
			return checkNoPointer(node.left) && checkNoPointer(node.right)
		case notNode:
			return checkNoPointer(node.node)
		case conditionNode:
			return true
		default:
			panic(fmt.Sprintf("Unknown node type %T", node))
		}

	}

	for _, test := range listToTreeTests {
		node := listToTree(test.and, test.inNodes)
		if node.String() != test.output {
			t.Fatalf("testcase: %v andflag: %v expected output: %s actual output: %s", test.inNodes, test.and, test.output, node.String())
		}
		if !checkNoPointer(node) {
			t.Fatalf("testcase: %v andflag: %v contains pointers somewhere in the tree", test.inNodes, test.and)
		}
	}
}
