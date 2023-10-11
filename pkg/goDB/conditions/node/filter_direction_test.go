package node

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

var isDirectionConditionTest = []struct {
	node        conditionNode
	expectedErr error
}{
	{node: conditionNode{attribute: "dir", comparator: "=", value: "in"}, expectedErr: errMisplacedDirectionFilterCondition},
	{node: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, expectedErr: nil},
}

func TestIsDirectionCondition(t *testing.T) {
	for _, test := range isDirectionConditionTest {
		n, err := isDirectionCondition(test.node)

		require.Equalf(t, err, test.expectedErr, "Expected error: %s Actual error: %s\n", test.expectedErr, err)
		if err == nil {
			require.Equalf(t, n.String(), test.node.String(), "Expected node: %s  Returned node:%s \n", n.String(), test.node.String())
		}
	}
}

var extractDirectionFilterTest = []struct {
	node        Node
	expectedErr error
}{
	{node: conditionNode{attribute: "dir", comparator: "=", value: "in"}, expectedErr: nil},
	{node: conditionNode{attribute: "dir", comparator: "=", value: "out"}, expectedErr: nil},
	{node: conditionNode{attribute: "dir", comparator: "=", value: "uni"}, expectedErr: nil},
	{node: conditionNode{attribute: "dir", comparator: "=", value: "bi"}, expectedErr: nil},
	{node: conditionNode{attribute: "dir", comparator: "!=", value: "bi"}, expectedErr: fmt.Errorf(unsupportedDirectionFilterComparatorStr, "!=")},
	{node: conditionNode{attribute: "dir", comparator: "=", value: "unknown"}, expectedErr: fmt.Errorf(unsupportedDirectionFilterStr, "unknown")},
	{node: notNode{node: conditionNode{attribute: "dir", comparator: "=", value: "unknown"}}, expectedErr: errNoFilter},
}

func TestExtractDirectionFilter(t *testing.T) {
	for _, test := range extractDirectionFilterTest {
		_, err := extractDirectionFilter(test.node)
		if err != nil {
			require.Equalf(t, test.expectedErr, err, "Expected error: %s Actual error: %s\n", test.expectedErr, err)
		}
	}
}

const conditionNone = "  "

var splitOffDirectionFilterTest = []struct {
	node                    Node
	expectedCondition       Node
	expectedFilterCondition string
	expectedErr             error
}{
	// only traffic filtering condition
	{node: conditionNode{attribute: "dir", comparator: "=", value: "in"}, expectedCondition: nil,
		expectedFilterCondition: "dir = in", expectedErr: errEmptyConditional},
	{node: conditionNode{attribute: "direction", comparator: "=", value: "inbound"}, expectedCondition: nil,
		expectedFilterCondition: "dir = inbound", expectedErr: errEmptyConditional},
	// valid filter within first term of top-level conjunction
	{node: andNode{left: conditionNode{attribute: "dir", comparator: "=", value: "out"}, right: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}},
		expectedCondition:       conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		expectedFilterCondition: "dir = out", expectedErr: nil},
	{node: andNode{left: conditionNode{attribute: "direction", comparator: "=", value: "outbound"}, right: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}},
		expectedCondition:       conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		expectedFilterCondition: "direction = outbound", expectedErr: nil},
	// valid filter within second term of top-level conjunction
	{node: andNode{left: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "dir", comparator: "=", value: "uni"}},
		expectedCondition:       conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		expectedFilterCondition: "dir = uni", expectedErr: nil},
	{node: andNode{left: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "dir", comparator: "=", value: "unidirectional"}},
		expectedCondition:       conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		expectedFilterCondition: "dir = unidirectional", expectedErr: nil},
	{node: andNode{left: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "dir", comparator: "=", value: "bidirectional"}},
		expectedCondition:       conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		expectedFilterCondition: "dir = bidirectional", expectedErr: nil},
	// invalid filter (2 traffic filters provided)
	{node: andNode{left: conditionNode{attribute: "dir", comparator: "=", value: "uni"},
		right: conditionNode{attribute: "dir", comparator: "=", value: "bi"}},
		expectedCondition:       nil,
		expectedFilterCondition: "", expectedErr: errMultipleDirectionFilterConditions},
	// invalid filter (2 traffic filters provided, of which one is nested)
	{node: andNode{left: conditionNode{attribute: "dir", comparator: "=", value: "uni"},
		right: andNode{left: conditionNode{attribute: "dip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "dir", comparator: "=", value: "bi"}}},
		expectedCondition:       nil,
		expectedFilterCondition: "", expectedErr: errMultipleDirectionFilterConditions},
	// simple condition without any traffic filter => valid
	{node: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		expectedCondition:       conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		expectedFilterCondition: conditionNone, expectedErr: nil},
	// disjunctive condition with traffic filter => invalid
	{node: orNode{left: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "dir", comparator: "=", value: "in"}},
		expectedCondition:       nil,
		expectedFilterCondition: "", expectedErr: errMisplacedDirectionFilterCondition},
	// conjunctive condition without any traffic filter => valid
	{node: andNode{left: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}},
		expectedCondition:       andNode{left: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}},
		expectedFilterCondition: conditionNone, expectedErr: nil},
	// conjunctive condition with misplaced traffic filter => invalid
	{node: andNode{left: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		right: orNode{left: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "dir", comparator: "=", value: "in"}}},
		expectedCondition:       nil,
		expectedFilterCondition: "", expectedErr: errMisplacedDirectionFilterCondition},
}

func TestSplitOffDirectionFilter(t *testing.T) {
	for _, test := range splitOffDirectionFilterTest {
		condition, valFilterNode, err := splitOffDirectionFilter(test.node)
		if err != nil {
			require.Equalf(t, test.expectedErr, err, "Expected error: %s Actual error: %s\n", test.expectedErr, err)
		}
		if err == nil {
			if valFilterNode.String() != test.expectedFilterCondition {
				t.Fatalf("Expected filter type: %s Got: %s", test.expectedFilterCondition, valFilterNode.String())
			}
			require.Equalf(t, condition.String(), test.expectedCondition.String(), "Expected Node: %s Got %s",
				test.expectedCondition.String(), condition.String())
		}
	}
}
