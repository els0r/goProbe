/////////////////////////////////////////////////////////////////////////////////
//
// DesugarConditional.go
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, January 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import "fmt"

// Returns a desugared version of the receiver.
func desugar(node Node) (Node, error) {
	return node.transform(desugarConditionNode)
}

func desugarConditionNode(node conditionNode) (Node, error) {
	helper := func(name, src, dst, comparator, value string) (Node, error) {
		var result Node
		if comparator != "=" && comparator != "!=" {
			return result, fmt.Errorf("Invalid comparison operator in %s condition: %s", name, comparator)
		}

		result = orNode{
			left: conditionNode{
				attribute:  src,
				comparator: "=",
				value:      value,
			},
			right: conditionNode{
				attribute:  dst,
				comparator: "=",
				value:      value,
			},
		}

		if comparator == "!=" {
			result = notNode{
				node: result,
			}
		}

		return result, nil
	}

	switch node.attribute {
	case "src":
		node.attribute = "sip"
	case "dst":
		node.attribute = "dip"
	case "host":
		return helper("host", "sip", "dip", node.comparator, node.value)
	case "net":
		return helper("net", "snet", "dnet", node.comparator, node.value)
	default:
		// nothing to do
	}

	return node, nil
}
