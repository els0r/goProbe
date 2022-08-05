/////////////////////////////////////////////////////////////////////////////////
//
// Conditional.go
//
// Main file for conditional handling.
// In goProbe/goQuery lingo, a conditional is an expression like
// "sip = 127.0.0.1 | !(host = 192.168.178.1)".
// A conditional is built of logical operators and conditions such as "sip = 127.0.0.1"
// or "host = 192.168.178.1".
//
// Interface Node represents conditional ASTs. The files TokenizeConditional.go,
// ParseConditional.go, InstrumentConditional.go contain more specialized functionality
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
	"fmt"
	"time"
)

// ParseAndInstrumentConditional parses and instruments the given conditional string for evaluation.
// This is the main external function related to conditionals.
func ParseAndInstrumentConditional(conditional string, dnsTimeout time.Duration) (Node, error) {
	tokenList, err := TokenizeConditional(conditional)
	if err != nil {
		return nil, err
	}

	conditionalNode, err := parseConditional(tokenList)
	if err != nil {
		return nil, err
	}

	if conditionalNode != nil {
		if conditionalNode, err = desugar(conditionalNode); err != nil {
			return nil, err
		}

		if conditionalNode, err = resolve(conditionalNode, dnsTimeout); err != nil {
			return nil, err
		}

		conditionalNode = negationNormalForm(conditionalNode)

		if conditionalNode, err = instrument(conditionalNode); err != nil {
			return nil, err
		}
	}

	return conditionalNode, nil
}

// Node describes an AST node for the conditional grammar
// This interface is not meant to be implemented by structs
// outside of this package.
type Node interface {
	fmt.Stringer

	// Traverses the AST in DFS order and replaces each conditionNode
	// (i.e. each leaf) with the output of the argument function.
	// If the argument function returns an error, it is passed through
	// to the caller.
	transform(func(conditionNode) (Node, error)) (Node, error)

	// Evaluates the conditional. Make sure that you called
	// instrument before calling this.
	evaluate(*ExtraKey) bool

	// Returns the set of attributes used in the conditional.
	attributes() map[string]struct{}
}

type conditionNode struct {
	attribute    string
	comparator   string
	value        string
	currentValue []byte
	compareValue func(*ExtraKey) bool
}

func newConditionNode(attribute, comparator, value string) conditionNode {
	return conditionNode{attribute, comparator, value, nil, nil}
}
func (n conditionNode) String() string {
	return fmt.Sprintf("%s %s %s", n.attribute, n.comparator, n.value)
}
func (n conditionNode) transform(transformer func(conditionNode) (Node, error)) (Node, error) {
	return transformer(n)
}
func (n conditionNode) desugar() (Node, error) {
	return desugarConditionNode(n)
}
func (n conditionNode) instrument() (Node, error) {
	err := generateCompareValue(&n)
	return n, err
}
func (n conditionNode) evaluate(comparisonValue *ExtraKey) bool {
	return n.compareValue(comparisonValue)
}
func (n conditionNode) attributes() map[string]struct{} {
	return map[string]struct{}{
		n.attribute: {},
	}
}

type notNode struct {
	node Node
}

func (n notNode) String() string {
	var s string
	if n.node == nil {
		s = "<nil>"
	} else {
		s = n.node.String()
	}
	return fmt.Sprintf("!(%s)", s)
}
func (n notNode) transform(transformer func(conditionNode) (Node, error)) (Node, error) {
	var err error
	n.node, err = n.node.transform(transformer)
	return n, err
}
func (n notNode) evaluate(comparisonValue *ExtraKey) bool {
	return !n.node.evaluate(comparisonValue)
}
func (n notNode) attributes() map[string]struct{} {
	return n.node.attributes()
}

type andNode struct {
	left  Node
	right Node
}

func (n andNode) String() string {
	var sl, sr string
	if n.left == nil {
		sl = "<nil>"
	} else {
		sl = n.left.String()
	}
	if n.right == nil {
		sr = "<nil>"
	} else {
		sr = n.right.String()
	}
	return fmt.Sprintf("(%s & %s)", sl, sr)
}
func (n andNode) transform(transformer func(conditionNode) (Node, error)) (Node, error) {
	var err error
	n.left, err = n.left.transform(transformer)
	if err != nil {
		return nil, err
	}
	n.right, err = n.right.transform(transformer)
	return n, err
}
func (n andNode) evaluate(comparisonValue *ExtraKey) bool {
	return n.left.evaluate(comparisonValue) && n.right.evaluate(comparisonValue)
}
func (n andNode) attributes() map[string]struct{} {
	result := n.left.attributes()
	for attribute := range n.right.attributes() {
		result[attribute] = struct{}{}
	}
	return result
}

type orNode struct {
	left  Node
	right Node
}

func (n orNode) String() string {
	var sl, sr string
	if n.left == nil {
		sl = "<nil>"
	} else {
		sl = n.left.String()
	}
	if n.right == nil {
		sr = "<nil>"
	} else {
		sr = n.right.String()
	}
	return fmt.Sprintf("(%s | %s)", sl, sr)
}
func (n orNode) transform(transformer func(conditionNode) (Node, error)) (Node, error) {
	var err error
	n.left, err = n.left.transform(transformer)
	if err != nil {
		return nil, err
	}
	n.right, err = n.right.transform(transformer)
	return n, err
}
func (n orNode) evaluate(comparisonValue *ExtraKey) bool {
	return n.left.evaluate(comparisonValue) || n.right.evaluate(comparisonValue)
}
func (n orNode) attributes() map[string]struct{} {
	result := n.left.attributes()
	for attribute := range n.right.attributes() {
		result[attribute] = struct{}{}
	}
	return result
}

// Brings a conditional ast tree into negation normal form.
// (See https://en.wikipedia.org/wiki/Negation_normal_form for an in-depth explanation)
// The gist of it is: Bringing the tree into negation normal removes all notNodes from
// the tree and the result is logically equivalent to the input.
// For example, "!((sip = 127.0.0.1 & dip = 127.0.0.1) | dport = 80)" is
// converted into "(sip != 127.0.0.1 | dip != 127.0.0.1) & dport != 80".
func negationNormalForm(node Node) Node {
	var helper func(Node, bool) Node
	helper = func(node Node, negate bool) Node {
		switch node := node.(type) {
		default:
			panic(fmt.Sprintf("Node unexpectly has type %T", node))
		case conditionNode:
			if negate {
				switch node.comparator {
				default:
					panic(fmt.Sprintf("Unknown comparison operator %s", node.comparator))
				case "=":
					node.comparator = "!="
				case "!=":
					node.comparator = "="
				case "<":
					node.comparator = ">="
				case ">":
					node.comparator = "<="
				case "<=":
					node.comparator = ">"
				case ">=":
					node.comparator = "<"
				}
				return node
			}
			return node
		case andNode:
			if negate {
				return orNode{
					left:  helper(node.left, true),
					right: helper(node.right, true),
				}
			}
			return andNode{
				left:  helper(node.left, false),
				right: helper(node.right, false),
			}
		case orNode:
			if negate {
				return andNode{
					left:  helper(node.left, true),
					right: helper(node.right, true),
				}
			}
			return orNode{
				left:  helper(node.left, false),
				right: helper(node.right, false),
			}
		case notNode:
			return helper(node.node, !negate)
		}
	}
	return helper(node, false)
}
