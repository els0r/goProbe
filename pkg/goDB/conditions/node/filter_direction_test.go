package node

import (
	"errors"
	"fmt"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"reflect"
	"testing"
)

var isDirectionConditionTest = []struct {
	node        conditionNode
	expectedErr error
}{
	{node: conditionNode{attribute: "dir", comparator: "=", value: "in"}, expectedErr: errMultipleDirectionFilterConditions},
	{node: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, expectedErr: nil},
}

func TestIsDirectionCondition(t *testing.T) {
	for _, test := range isDirectionConditionTest {
		n, err := isDirectionCondition(test.node)
		if !errors.Is(err, test.expectedErr) {
			t.Fatalf("Expected error: %s Actual error: %s\n", test.expectedErr, err)
		}
		if err == nil && n.String() != test.node.String() {
			t.Fatalf("Expected node: %s  Returned node:%s \n", n.String(), test.node.String())
		}
	}
}

var extractDirectionFilterTest = []struct {
	node           Node
	expectedFilter hashmap.ValFilter
	expectedErr    error
}{
	{node: conditionNode{attribute: "dir", comparator: "=", value: "in"}, expectedFilter: types.Counters.IsOnlyInbound, expectedErr: nil},
	{node: conditionNode{attribute: "dir", comparator: "=", value: "out"}, expectedFilter: types.Counters.IsOnlyOutbound, expectedErr: nil},
	{node: conditionNode{attribute: "dir", comparator: "=", value: "uni"}, expectedFilter: types.Counters.IsUnidirectional, expectedErr: nil},
	{node: conditionNode{attribute: "dir", comparator: "=", value: "bi"}, expectedFilter: types.Counters.IsBidirectional, expectedErr: nil},
	{node: conditionNode{attribute: "dir", comparator: "!=", value: "bi"}, expectedFilter: nil, expectedErr: fmt.Errorf(unsupportedDirectionFilterComparatorStr, "!=")},
	{node: conditionNode{attribute: "dir", comparator: "=", value: "unknown"}, expectedFilter: nil, expectedErr: fmt.Errorf(unsupportedDirectionFilterStr, "unknown")},
	{node: notNode{node: conditionNode{attribute: "dir", comparator: "=", value: "unknown"}}, expectedFilter: nil, expectedErr: nil},
}

func TestExtractDirectionFilter(t *testing.T) {
	for _, test := range extractDirectionFilterTest {
		filter, err := extractDirectionFilter(test.node)
		if err != nil && err.Error() != test.expectedErr.Error() {
			t.Fatalf("Expected error: %s Actual error: %s\n", test.expectedErr, err)
		}
		if err == nil && reflect.ValueOf(filter) != reflect.ValueOf(test.expectedFilter) {
			t.Fatalf("Expected filter type: %s Got: %s", reflect.TypeOf(test.expectedFilter), reflect.TypeOf(filter))
		}
	}
}

var splitOffDirectionFilterTest = []struct {
	node              Node
	expectedCondition Node
	expectedFilter    hashmap.ValFilter
	expectedErr       error
}{
	// only traffic filtering condition
	{node: conditionNode{attribute: "dir", comparator: "=", value: "in"}, expectedCondition: nil,
		expectedFilter: types.Counters.IsOnlyInbound, expectedErr: errEmptyConditional},
	// valid filter within first term of top-level conjunction
	{node: andNode{left: conditionNode{attribute: "dir", comparator: "=", value: "out"}, right: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}},
		expectedCondition: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		expectedFilter:    types.Counters.IsOnlyOutbound, expectedErr: nil},
	// valid filter within second term of top-level conjunction
	{node: andNode{left: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "dir", comparator: "=", value: "uni"}},
		expectedCondition: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		expectedFilter:    types.Counters.IsUnidirectional, expectedErr: nil},
	// invalid filter (2 traffic filters provided)
	{node: andNode{left: conditionNode{attribute: "dir", comparator: "=", value: "uni"},
		right: conditionNode{attribute: "dir", comparator: "=", value: "bi"}},
		expectedCondition: nil,
		expectedFilter:    nil, expectedErr: errMultipleDirectionFilterConditions},
	// invalid filter (2 traffic filters provided, of which one is nested)
	{node: andNode{left: conditionNode{attribute: "dir", comparator: "=", value: "uni"},
		right: andNode{left: conditionNode{attribute: "dip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "dir", comparator: "=", value: "bi"}}},
		expectedCondition: nil,
		expectedFilter:    nil, expectedErr: errMultipleDirectionFilterConditions},
	// simple condition without any traffic filter => valid
	{node: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		expectedCondition: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"},
		expectedFilter:    nil, expectedErr: nil},
	// conjunctive condition without any traffic filter => valid
	{node: andNode{left: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}},
		expectedCondition: andNode{left: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}, right: conditionNode{attribute: "sip", comparator: "=", value: "127.0.0.1"}},
		expectedFilter:    nil, expectedErr: nil},
}

func TestSplitOffDirectionFilter(t *testing.T) {
	for _, test := range splitOffDirectionFilterTest {
		condition, valFilterNode, err := splitOffDirectionFilter(test.node)
		if err != nil && err.Error() != test.expectedErr.Error() {
			t.Fatalf("Expected error: %s Actual error: %s\n", test.expectedErr, err)
		}

		if err == nil && reflect.ValueOf(valFilterNode.ValFilter) != reflect.ValueOf(test.expectedFilter) {
			t.Fatalf("Expected filter type: %s Got: %s", reflect.TypeOf(test.expectedFilter), reflect.TypeOf(valFilterNode.ValFilter))
		}
		if err == nil && condition.String() != test.expectedCondition.String() {
			t.Log("Incorrect split off of condition")
			t.Logf("Expected: %s", test.expectedCondition.String())
			t.Logf("Got: %s", condition.String())
			t.Fatal()
		}
	}
}
