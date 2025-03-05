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

package node

import (
	"fmt"

	"github.com/els0r/goProbe/v4/pkg/types"
)

// Returns a desugared version of the receiver.
func desugar(node Node) (Node, error) {
	return node.transform(desugarConditionNode)
}

func desugarConditionNode(node conditionNode) (Node, error) {
	helper := func(name, src, dst, comparator, value string) (Node, error) {
		var result Node
		if comparator != "=" && comparator != "!=" {
			return result, fmt.Errorf("invalid comparison operator in %s condition: %s", name, comparator)
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

	// map aliases to proper attribute names
	switch node.attribute {
	case "src":
		node.attribute = types.SIPName
	case "dst":
		node.attribute = types.DIPName
	case "port":
		node.attribute = types.DportName
	case "ipproto", "protocol":
		node.attribute = types.ProtoName
	case "host":
		return helper("host", types.SIPName, types.DIPName, node.comparator, node.value)
	case "net":
		return helper("net", "snet", "dnet", node.comparator, node.value)
	default:
		// nothing to do
	}

	return node, nil
}
