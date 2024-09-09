package types

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type testDefinitionWithError struct {
	name          string
	input         string
	exepctedError error
}

func TestInterfaceValidation(t *testing.T) {

	// create args
	var tests = []struct {
		iface       string
		expectedErr error
	}{
		{
			"",
			errors.New("interface list contains empty interface name"),
		},
		{
			"eth/0",
			errors.New("interface name `eth/0` is invalid"),
		},
		{
			"eth 0",
			errors.New("interface name `eth 0` is invalid"),
		},
		{
			"thisinterfacenameisfartoolongtobesupported",
			errors.New("interface name `thisinterfacenameisfartoolongtobesupported` is invalid"),
		},
		{
			"eth.15",
			nil,
		},
		{
			"eth:0",
			nil,
		},
		{
			"!t4",
			nil,
		},
		{
			"eth[0-9]|t[0-9]",
			errors.New("interface name `eth[0-9]|t[0-9]` is invalid"),
		},
		{
			"%,%",
			errors.New("interface name `%,%` is invalid"),
		},
	}

	// run table-driven test
	for _, test := range tests {
		t.Run(test.iface, func(t *testing.T) {
			err := ValidateIfaceName(test.iface)
			if test.expectedErr != nil {
				if err == nil || err.Error() != test.expectedErr.Error() {
					t.Fatalf("unexpected result for interface name validation: %s", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected result for interface name validation: %s", err)
				}
			}
		})
	}
}

func TestSplitAndValidateIFaces(t *testing.T) {

	var testCases = []struct {
		name             string
		input            string
		outputPosFilters []string
		outputNegFilters []string
		expectedError    error
	}{
		{
			name:             "valid iface names",
			input:            "eth0,eth1,!eth2",
			outputPosFilters: []string{"eth0", "eth1"},
			outputNegFilters: []string{"eth2"},
			expectedError:    nil,
		},
		{
			name:             "invalid due to empty interface names",
			input:            ",,,",
			outputPosFilters: nil,
			outputNegFilters: nil,
			expectedError:    errors.New("interface list contains empty interface name"),
		},
		{
			name:             "invalid list of regexp",
			input:            "eth[0-4],t[0-4]",
			outputPosFilters: nil,
			outputNegFilters: nil,
			expectedError:    errors.New("interface name `eth[0-4]` is invalid"),
		},
		{
			name:             "invalid due to special characters",
			input:            "%,*,",
			outputPosFilters: nil,
			outputNegFilters: nil,
			expectedError:    errors.New("interface name `%` is invalid"),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			actualPos, actualNeg, actualError := ValidateAndSeparateFilters(test.input)
			if actualError != nil {
				require.EqualValues(t, test.expectedError, actualError)
			} else {
				require.EqualValues(t, test.outputPosFilters, actualPos)
				require.EqualValues(t, test.outputNegFilters, actualNeg)
			}
		})
	}
}

func TestValidateIRegExp(t *testing.T) {
	var testCases = []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "valid iface regexp",
			input:  "eth[0-4]",
			output: "eth[0-4]",
		},
		{
			name:   "invalid iface regexp",
			input:  "/\\",
			output: "error parsing regexp: trailing backslash at end of expression: ``",
		},
		{
			name:   "invalid iface regexp",
			input:  "",
			output: "interface regexp is empty",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			actual, actualError := ValidateRegExp(test.input)
			if actual == nil { // validation returns nil when all is ok.
				require.EqualValues(t, test.output, actualError.Error())
			} else {
				require.EqualValues(t, test.output, actual.String())
			}
		})
	}
}
