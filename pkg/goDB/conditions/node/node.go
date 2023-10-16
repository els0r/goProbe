/////////////////////////////////////////////////////////////////////////////////
//
// node.go
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

package node

import (
	"errors"
	"fmt"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/conditions"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
)

const resNil = "<nil>"

// ParseAndInstrument parses and instruments the given conditional string for evaluation.
// This is the main external function related to conditionals.
func ParseAndInstrument(conditional string, dnsTimeout time.Duration) (Node, *ValFilterNode, error) {
	tokens, err := conditions.Tokenize(conditional)
	if err != nil {
		return nil, nil, err
	}

	conditionalNode, err := parseConditional(tokens)
	if err != nil && !errors.Is(err, errEmptyConditional) {
		return nil, nil, err
	}
	conditionalNode, valFilterNode, err := splitOffDirectionFilter(conditionalNode)
	if err != nil && !errors.Is(err, errEmptyConditional) {
		return nil, nil, err
	}

	if conditionalNode != nil {
		if conditionalNode, err = desugar(conditionalNode); err != nil {
			return nil, nil, err
		}

		if conditionalNode, err = resolve(conditionalNode, dnsTimeout); err != nil {
			return nil, nil, err
		}

		conditionalNode = negationNormalForm(conditionalNode)

		if conditionalNode, err = instrument(conditionalNode); err != nil {
			return nil, nil, err
		}
	}

	return conditionalNode, &valFilterNode, nil
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
	Evaluate(types.Key) bool

	// Returns the set of attributes used in the conditional.
	Attributes() map[string]types.IPVersion
}

type conditionNode struct {
	attribute    string
	comparator   string
	value        string
	ipVersion    types.IPVersion
	currentValue []byte
	compareValue func(types.Key) bool
}

func newConditionNode(attribute, comparator, value string) conditionNode {
	return conditionNode{attribute, comparator, value, types.IPVersionNone, nil, nil}
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
func (n conditionNode) Evaluate(comparisonValue types.Key) bool {
	return n.compareValue(comparisonValue)
}
func (n conditionNode) Attributes() map[string]types.IPVersion {
	return map[string]types.IPVersion{
		n.attribute: n.ipVersion,
	}
}

type notNode struct {
	node Node
}

func (n notNode) String() string {
	var s string
	if n.node == nil {
		s = resNil
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
func (n notNode) Evaluate(comparisonValue types.Key) bool {
	return !n.node.Evaluate(comparisonValue)
}
func (n notNode) Attributes() map[string]types.IPVersion {
	return n.node.Attributes()
}

type andNode struct {
	left  Node
	right Node
}

func (n andNode) String() string {
	var sl, sr string
	if n.left == nil {
		sl = resNil
	} else {
		sl = n.left.String()
	}
	if n.right == nil {
		sr = resNil
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
func (n andNode) Evaluate(comparisonValue types.Key) bool {
	return n.left.Evaluate(comparisonValue) && n.right.Evaluate(comparisonValue)
}
func (n andNode) Attributes() map[string]types.IPVersion {
	result := n.left.Attributes()
	for attribute, ipVersion := range n.right.Attributes() {
		result[attribute] = result[attribute].Merge(ipVersion)
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
		sl = resNil
	} else {
		sl = n.left.String()
	}
	if n.right == nil {
		sr = resNil
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

func (n orNode) Evaluate(comparisonValue types.Key) bool {
	return n.left.Evaluate(comparisonValue) || n.right.Evaluate(comparisonValue)
}

func (n orNode) Attributes() map[string]types.IPVersion {
	result := n.left.Attributes()
	for attribute, ipVersion := range n.right.Attributes() {
		result[attribute] = result[attribute].Merge(ipVersion)
	}
	return result
}

// ValFilterNode describes a node representing a ValFilter.
// LeftNode is true if the ValFilterNode occurs on the left side
// of a conjunction (andNode) and false if it occurs on the
// right side.
type ValFilterNode struct {
	conditionNode
	FilterType string
	ValFilter  hashmap.ValFilter
	LeftNode   bool
}

func (filter *ValFilterNode) setValFilterValues(cn conditionNode, ft string,
	vf hashmap.ValFilter, ln bool) {
	filter.conditionNode = cn
	filter.FilterType = ft
	filter.ValFilter = vf
	filter.LeftNode = ln
}

// QueryConditionalString constructs the conditional string shown in the
// "Conditions" output field based on the query conditions and the query filter
func QueryConditionalString(conditionalNode Node, filterNode Node) string {
	valFilterNode, ok := filterNode.(*ValFilterNode)
	if !ok || valFilterNode.FilterType == types.FilterKeywordNone {
		if conditionalNode == nil {
			return ""
		}
		return conditionalNode.String()
	}
	if conditionalNode == nil {
		return valFilterNode.String()
	}
	var n andNode
	if valFilterNode.LeftNode {
		n = andNode{left: valFilterNode.conditionNode, right: conditionalNode}
	} else {
		n = andNode{left: conditionalNode, right: valFilterNode.conditionNode}
	}
	return n.String()
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
