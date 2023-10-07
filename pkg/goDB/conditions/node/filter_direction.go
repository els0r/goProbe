package node

import (
	"errors"
	"fmt"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
)

var (
	// errMultipleDirectionFilterConditions indicates that
	// the specified condition contains too many traffic direction
	// filters
	errMultipleDirectionFilterConditions = errors.New("multiple direction filters specified")

	// errMisplacedDirectionFilterCondition indicates that
	// the specified condition contains a misplaced traffic direction
	// filter
	errMisplacedDirectionFilterCondition = errors.New("misplaced direction filter")

	unsupportedDirectionFilterComparatorStr = "unsupported direction filter comparator: %s"
	unsupportedDirectionFilterStr           = "unsupported direction filter: %s (use one of 'in', 'out', 'uni', 'bi')"
)

// isDirectionCondition returns an error if node represents a
// direction filter condition, and the node itself otherwise.
// It is used in combination with node.transform() in order
// to ensure direction filter conditions occur only at the
// top level of the node or the second level in case the
// top level represents a conjunction (AND).
func isDirectionCondition(node conditionNode) (Node, error) {
	if node.attribute == types.FilterKeywordDirection {
		return nil, errMisplacedDirectionFilterCondition
	}
	return node, nil
}

// extractDirectionFilter returns a ValFilter if node represents a valid direction filter condition.
// If node represents an invalid direction filter (e.g., unsupported direction filter comparator),
// nil is returned with an error describing the invalidity reason.
// If the node does not contain a filter condition, nil is returned without any error.
func extractDirectionFilter(node Node) (hashmap.ValFilter, error) {
	switch node := node.(type) {
	case conditionNode:
		if node.attribute != types.FilterKeywordDirection && node.attribute != types.FilterKeywordDirectionSugared {
			return nil, nil
		}
		if node.comparator != "=" {
			return nil, fmt.Errorf(unsupportedDirectionFilterComparatorStr, node.comparator)
		}
		var filter hashmap.ValFilter
		switch types.FilterTypeDirection(node.value) {
		case types.FilterTypeDirectionIn, types.FilterTypeDirectionInSugared:
			filter = types.Counters.IsOnlyInbound
		case types.FilterTypeDirectionOut, types.FilterTypeDirectionOutSugared:
			filter = types.Counters.IsOnlyOutbound
		case types.FilterTypeDirectionUni, types.FilterTypeDirectionUniSugared:
			filter = types.Counters.IsUnidirectional
		case types.FilterTypeDirectionBi, types.FilterTypeDirectionBiSugared:
			filter = types.Counters.IsBidirectional
		default:
			return nil, fmt.Errorf(unsupportedDirectionFilterStr, node.value)
		}
		return filter, nil
	default:
		return nil, nil
	}
}

// splitOffDirectionFilter splits off the traffic direction filter
// from the node, and returns the node without the direction filter,
// and a separate ValFilterNode representing the direction filter.
// The direction filter placement inside the node is being validated.
// If there is no direction filter included in node, it returns the node
// and a ValFilterNode with FilterType FilterKeywordNone.
func splitOffDirectionFilter(node Node) (Node, ValFilterNode, error) {
	valFilterNode := ValFilterNode{FilterType: types.FilterKeywordNone}
	switch node := node.(type) {
	case conditionNode:
		filter, err := extractDirectionFilter(node)
		if err != nil {
			return nil, valFilterNode, err
		}
		if filter == nil {
			return node, valFilterNode, nil
		}
		valFilterNode.FilterType = types.FilterKeywordDirection
		valFilterNode.conditionNode = node
		valFilterNode.LeftNode = false
		valFilterNode.ValFilter = filter
		return nil, valFilterNode, errEmptyConditional
	case andNode:
		nodes := [2]Node{node.left, node.right}
		filters := [2]hashmap.ValFilter{nil, nil}
		for i := 0; i < 2; i++ {
			n := nodes[i]
			filter, err := extractDirectionFilter(n)
			if err != nil {
				continue
			}
			filters[i] = filter
		}
		// none of the two child nodes represent direction conditions
		// => nothing to split off
		if filters[0] == nil && filters[1] == nil {
			_, err := node.transform(isDirectionCondition)
			if err != nil {
				return nil, valFilterNode, err
			}
			return node, valFilterNode, nil
		}
		// both of the two child nodes represent direction conditions
		// => invalid filter condition string
		// e.g. "dir = in and dir = out"
		if filters[0] != nil && filters[1] != nil {
			return nil, valFilterNode, errMultipleDirectionFilterConditions
		}

		// check that the conjunction child node that does not represent a direction filter
		// condition also does not contain a filtering condition at a lower level
		// e.g. forbidden: "dir = in and ( sip = 1.2.3.4 and dir = out )"
		i := 0
		if filters[i] != nil {
			i = 1
		}
		n := nodes[i]
		_, err := n.transform(isDirectionCondition)
		if err != nil {
			return nil, valFilterNode, errMultipleDirectionFilterConditions
		}
		valFilterNode.FilterType = types.FilterKeywordDirection
		valFilterNode.conditionNode = nodes[1-i].(conditionNode)
		valFilterNode.LeftNode = i == 1
		valFilterNode.ValFilter = filters[1-i]
		return n, valFilterNode, nil
	default:
		if node == nil {
			return node, valFilterNode, nil
		}
		_, err := node.transform(isDirectionCondition)
		if err != nil {
			return nil, valFilterNode, err
		}
		return node, valFilterNode, nil
	}
}
