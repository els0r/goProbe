package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type conditionalTest struct {
	in                   []string
	expectedNSuggestions int
}

func testConditionals(t *testing.T, tests []conditionalTest) {
	for _, test := range tests {
		suggs := conditional(test.in)
		nSuggs := len(suggs)
		require.Equalf(t, test.expectedNSuggestions, nSuggs, "Expected: %d Got: %d", test.expectedNSuggestions, nSuggs)
	}
}

func TestConditionalsBasic(t *testing.T) {
	var conditionalTestsBasic = []conditionalTest{
		{[]string{""}, 15},
		{[]string{"!"}, 12},
		{[]string{"goquery", "-c", "d"}, 6},
		{[]string{"goquery", "-c", "di"}, 3},
		{[]string{"goquery", "-c", "dir"}, 2},
		{[]string{"goquery", "-c", "ds"}, 2},
		{[]string{"goquery", "-c", "dip"}, 2},
	}

	testConditionals(t, conditionalTestsBasic)
}

func TestConditionalsStructure(t *testing.T) {
	var conditionalStructureTests = []conditionalTest{
		{[]string{"goquery", "-c", "dire"}, 8},        // direction = {direction values}
		{[]string{"goquery", "-c", "direction ="}, 8}, // direction = {direction values}
		{[]string{"goquery", "-c", "dir = i"}, 2},
		{[]string{"goquery", "-c", "dir = in"}, 2},

		// Complete inbound + only suggest & + don't suggest another dir keyword.
		{[]string{"goquery", "-c", "dir = inb"}, 14},
		{[]string{"goquery", "-c", "sip = 127.0.0.1 & dir = inb"}, 1},
		{[]string{"goquery", "-c", "sip = 127.0.0.1 & dir = inb"}, 1},

		// Suggest dir directly after top-level &.
		{[]string{"goquery", "-c", "sip = 127.0.0.1 & "}, 15},
		{[]string{"goquery", "-c", "(sip = 127.0.0.1 & dport = 22) & "}, 15},
		// Don't suggest dir after non-top-level &.
		{[]string{"goquery", "-c", "sip = 127.0.0.1 & dport = 22 & "}, 13},
		{[]string{"goquery", "-c", "sip = 127.0.0.1 & dport = 22 & dir = "}, 2},

		// Don't suggest dir after top-level |.
		{[]string{"goquery", "-c", "sip = 127.0.0.1 | "}, 13},

		// Don't suggest after invalid condition strings.
		{[]string{"goquery", "-c", "sip = 127.0.0.1 & dport = 22 & dir = "}, 2},
		{[]string{"goquery", "-c", "sip = 127.0.0.1 | dir ="}, 2},
		{[]string{"goquery", "-c", "sip = 127.0.0.1 | dir "}, 2},
	}

	testConditionals(t, conditionalStructureTests)
}

func TestConditionalsEdgeCases(t *testing.T) {
	var conditionalEdgeCasesTests = []conditionalTest{

		// Successful condition string termination.
		{[]string{"goquery", "-c", "sip = 127.0.0.1 & dir = inbound"}, 1},
		{[]string{"goquery", "-c", "sip = 127.0.0.1 & dir = inbound "}, 1},
		{[]string{"goquery", "-c", "dir = out & sip = 127.0.0.1"}, 1},
		{[]string{"goquery", "-c", "dir = out & (sip = 127.0.0.1 | dport = 22)"}, 1},
		{[]string{"goquery", "-c", "dir = out & (sip = 127.0.0.1 | dport = 22) "}, 1},

		// Do not terminate condition string.
		{[]string{"goquery", "-c", "dir = out & (sip = 127.0.0.1 |"}, 13},
		{[]string{"goquery", "-c", "dir = out "}, 13},
		{[]string{"goquery", "-c", "dir = inbound & ( dst = 127.0.0.1 & src = 127.0.0.1) &"}, 2},
		{[]string{"goquery", "-c", "dir = inbound & ( dst = 127.0.0.1 & src = 127.0.0.1) |"}, 2},
		{[]string{"goquery", "-c", "dir = inbound & ( dst = 127.0.0.1 & src = 127.0.0.1) & "}, 2},
		{[]string{"goquery", "-c", "dir = inbound & ( dst = 127.0.0.1 & src = 127.0.0.1) | "}, 2},

		// Invalid dir values (yields itself + "can't help" text).
		{[]string{"goquery", "-c", "dir = inn"}, 2},
		{[]string{"goquery", "-c", "sip = 127.0.0.1 & dir = inn"}, 2},
	}

	testConditionals(t, conditionalEdgeCasesTests)
}
