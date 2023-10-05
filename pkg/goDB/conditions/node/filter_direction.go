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

	// errNoFilter indicates that the node does not contain a filter condition
	errNoFilter = errors.New("Node does not contain a filter condition")

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
	if node.attribute == types.FilterKeywordDirection || node.attribute == types.FilterKeywordDirectionSugared {
		return nil, errMisplacedDirectionFilterCondition
	}
	return node, nil
}

// extractDirectionFilter returns a ValFilter if node represents a valid direction filter condition.
// If node represents an invalid direction filter (e.g., unsupported direction filter comparator),
// nil is returned with an error describing the invalidity reason.
// If the node does not contain a filter condition, nil is returned without any error.
func extractDirectionFilter(node Node) (hashmap.ValFilter, error) {
	if node, ok := node.(conditionNode); ok {
		if node.attribute != types.FilterKeywordDirection && node.attribute != types.FilterKeywordDirectionSugared {
			return nil, nil
		}
		if node.comparator != "=" {
			return nil, fmt.Errorf(unsupportedDirectionFilterComparatorStr, node.comparator)
		}
		var filter hashmap.ValFilter
		switch node.value {
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
	} else {
		return nil, nil
	}
}

// extractDirectionFiltersFromNodes extracts the ValFilters from a list of nodes.
// If the i-th node does not represent a ValFilter, the i-th entry of the output is nil.
func extractDirectionFiltersFromNodes(nodes []Node) []hashmap.ValFilter {
	filters := make([]hashmap.ValFilter, len(nodes))
	for i, node := range nodes {
		filter, _ := extractDirectionFilter(node)
		filters[i] = filter
	}
	return filters
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
		valFilterNode.setValFilterValues(node, types.FilterKeywordDirection, filter, false)
		return nil, valFilterNode, errEmptyConditional
	case andNode:

		// extract filters from both AND conditions
		nodes := []Node{node.left, node.right}
		filters := extractDirectionFiltersFromNodes(nodes)

		// none of the two child nodes represent direction filters
		// => check that the node does not contain any filter condition
		// at a lower level
		if filters[0] == nil && filters[1] == nil {
			_, err := node.transform(isDirectionCondition)
			if err != nil {
				return nil, valFilterNode, err
			}
			return node, valFilterNode, nil
		}

		// both AND conditions represent direction filters
		// (e.g. "dir = in and dir = out")
		// => invalid filter condition string
		if filters[0] != nil && filters[1] != nil {
			return nil, valFilterNode, errMultipleDirectionFilterConditions
		}

		// determine which AND condition represents a
		// direction filter
		filterIndex := 0
		if filters[filterIndex] == nil {
			filterIndex = 1
		}

		// check that the non-filter AND condition also does not
		// contain a direction filter at a lower level
		// (e.g. "dir = in & (sip = 1.2.3.4 & dir = out)")
		// => invalid filter condition string
		n := nodes[1-filterIndex]
		_, err := n.transform(isDirectionCondition)
		if err != nil {
			return nil, valFilterNode, errMultipleDirectionFilterConditions
		}

		// set the ValFilterNode values
		filterNode := nodes[filterIndex].(conditionNode)
		filter := filters[filterIndex]
		valFilterNode.setValFilterValues(filterNode, types.FilterKeywordDirection, filter, filterIndex == 0)
		return n, valFilterNode, nil
	case nil:
		return node, valFilterNode, nil
	default:
		_, err := node.transform(isDirectionCondition)
		if err != nil {
			return nil, valFilterNode, err
		}
		return node, valFilterNode, nil
	}
}
