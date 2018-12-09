/////////////////////////////////////////////////////////////////////////////////
//
// output_consistency_test.go
//
//
// Written by Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
)

const (
	// Constants used by output consistency testing
	OUTPUT_CONSISTENCY_DIR = "./output_consistency"
	CORRECT_OUTPUT_SUFFIX  = ".correctOutput.json"
	ARGS_SUFFIX            = ".args.json"
	// Used as a placeholder for the path to the test database inside .args.json files
	TESTDB_VARIABLE = "$TESTDB"
)

// Compares output of goQuery with known good outputs.
//
// Idea
//
// For the output consistency tests, the goal is to compare the output of the program
// (on a semantic level, so we can't just use string comparison) to known good outputs
// for certain sets of input parameters. This helps us ensure that we didn't break something
// in goQuery when we introduce future changes.
// When a test fails there are three possibilities:
// * The new behaviour of goQuery is incorrect
// * The old behaviour of goQuery is incorrect (unlikely)
// * The test itself is broken
// In either case, this is valuable information.
//
// Implementation
//
// Since there are many different combinations of command line arguments and
// goQuery outputs can quickly become rather large, we don't use table driven
// tests (where we specify each (arguments, expected output) pair in source
// code). Instead, we have a special directory that contains the tests. Each test
// consists of two files:
// 1. A .args.json file that contains command line arguments
// 2. A .correctOutput.json file that contains the correct output
//
// To run a test, we run goQuery with each argument list specified in the
// .args.json (there can be many!) file, take its output, and check whether it
// matches the .correctOutput.json file.
func TestOutputConsistency(t *testing.T) {
	fatalfWithBashCommand := func(arguments []string, msg string, args ...interface{}) {
		bashcommand := "./goQuery"
		for _, argument := range arguments {
			bashcommand += " "
			// crappy escaping, should be good enough for our purposes
			if strings.Contains(argument, " ") {
				bashcommand += "\"" + argument + "\""
			} else {
				bashcommand += argument
			}
		}
		t.Fatalf(fmt.Sprintf(msg, args...)+"\nBash command to reproduce: %s", bashcommand)
	}

	t.Parallel()

	checkDbExists(t, SMALL_GODB)

	testCases, err := testCases()
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range testCases {
		argumentFile := path.Join(OUTPUT_CONSISTENCY_DIR, testCase+ARGS_SUFFIX)
		expectedOutputFile := path.Join(OUTPUT_CONSISTENCY_DIR, testCase+CORRECT_OUTPUT_SUFFIX)

		// Read arguments
		argumentssJson, err := ioutil.ReadFile(argumentFile)
		if err != nil {
			t.Fatalf("Could not read argument file %s. Error: %s", argumentFile, err)
		}
		var argumentss [][]string
		err = json.Unmarshal(argumentssJson, &argumentss)
		if err != nil {
			t.Fatalf("Could not decode argument file %s. Error: %s", argumentFile, err)
		}

		// Read expected output
		expectedOutputJson, err := ioutil.ReadFile(expectedOutputFile)
		if err != nil {
			t.Fatalf("Could not read expected output file %s. Error: %s", expectedOutputFile, err)
		}
		var expectedOutput interface{}
		err = json.Unmarshal(expectedOutputJson, &expectedOutput)
		if err != nil {
			t.Fatalf("Could not decode expected output file %s. Error: %s", expectedOutputFile, err)
		}

		for i, arguments := range argumentss {

			// Replace all occurrences of TESTDB_VARIABLE with the path to the test database
			arguments = replaceTestDBVar(arguments)

			cmd := callMain(arguments...)
			actualOutputJson, err := cmd.Output()
			// We are running our real main() inside the test executable's main(). The latter always prints PASS\n
			// if there were no errors. This makes the JSON parser unhappy, so we remove it from the output.
			if len(actualOutputJson) < len("PASS\n") {
				fatalfWithBashCommand(arguments, "Something strange happened in testcase %s[%d].", testCase, i)
			} else {
				actualOutputJson = actualOutputJson[:len(actualOutputJson)-len("PASS\n")]
			}

			var actualOutput interface{}
			err = json.Unmarshal(actualOutputJson, &actualOutput)
			if err != nil {
				fmt.Println(string(actualOutputJson))
				fatalfWithBashCommand(arguments, "Failed to parse output from testcase %s[%d] as JSON: %s", testCase, i, err)
			}
			if !outputMatches(expectedOutput, actualOutput) {
				fatalfWithBashCommand(arguments, "Output from testcase %s[%d] doesn't match correct output", testCase, i)
			}
		}
	}
}

// Replace all occurrences of TESTDB_VARIABLE with the path to the test database
func replaceTestDBVar(arguments []string) (modifiedArguments []string) {
	modifiedArguments = make([]string, len(arguments))
	copy(modifiedArguments, arguments)
	for i := range modifiedArguments {
		if modifiedArguments[i] == TESTDB_VARIABLE {
			modifiedArguments[i] = SMALL_GODB
		}
	}
	return
}

// Validates the directry structure of OUTPUT_CONSISTENCY_DIR and finds all test cases in it.
// Each test case X consists of two files: X.correctOutput.json and X.args.json
// If we find a X.correctOutput.json file without an accompanying X.args.json file (or the other way around)
// we fail.
func testCases() (testCases []string, err error) {
	// Open file descriptor for OUTPUT_CONSISTENCY_DIR
	fd, err := os.Open(OUTPUT_CONSISTENCY_DIR)
	if err != nil {
		err = fmt.Errorf("Could not open directory %v: %v", OUTPUT_CONSISTENCY_DIR, err)
		return
	}
	// Get a list of ALL files in the directory, that's what the -1 is for.
	fileInfos, err := fd.Readdir(-1)
	if err != nil {
		err = fmt.Errorf("Could not enumerate directory %v: %v", OUTPUT_CONSISTENCY_DIR, err)
		return
	}

	// Map to keep track of whether (for each testcase) we have seen a correctOutput file AND an args file.
	seen := make(map[string]struct {
		correctOutput bool
		args          bool
	})

	// Loop over all entries of OUTPUT_CONSISTENCY_DIR and populate seen map.
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}

		name := fileInfo.Name()
		if strings.HasSuffix(name, ARGS_SUFFIX) {
			key := name[:len(name)-len(ARGS_SUFFIX)]
			seenElem := seen[key]
			seenElem.args = true
			seen[key] = seenElem
		} else if strings.HasSuffix(name, CORRECT_OUTPUT_SUFFIX) {
			key := name[:len(name)-len(CORRECT_OUTPUT_SUFFIX)]
			seenElem := seen[key]
			seenElem.correctOutput = true
			seen[key] = seenElem
		}
	}

	// Check validity condition, i.e. whether for each testcase we have both files.
	for testName, seenFiles := range seen {
		if !seenFiles.args {
			return nil, fmt.Errorf("File %v%v is missing", testName, ARGS_SUFFIX)
		}
		if !seenFiles.correctOutput {
			return nil, fmt.Errorf("File %v%v is missing", testName, CORRECT_OUTPUT_SUFFIX)
		}
	}

	// If we get here, the validity condition is satisfied; we return the list of test cases.
	for testName, _ := range seen {
		testCases = append(testCases, testName)
	}
	return
}

// Check whether the actual output and the expected output (both as interface{}s
// unmarshalled from JSON) match.
func outputMatches(expected, actual interface{}) (ok bool) {
	expectedMap, isMap := expected.(map[string]interface{})
	if !isMap {
		return false
	}

	actualMap, isMap := actual.(map[string]interface{})
	if !isMap {
		return false
	}

	if len(actualMap) != 4 || len(expectedMap) != 4 {
		return false
	}

	// ext_ips: we only check that these are present, but don't compare
	// them, because they vary from host to host.
	if _, isSlice := expectedMap["ext_ips"].([]interface{}); !isSlice {
		return false
	}

	if _, isSlice := actualMap["ext_ips"].([]interface{}); !isSlice {
		return false
	}

	// status
	if !reflect.DeepEqual(expectedMap["status"], actualMap["status"]) {
		return false
	}

	// summary
	// there are multiple ways to write the same interface list
	delete(expectedMap["summary"].(map[string]interface{}), "interface")
	delete(actualMap["summary"].(map[string]interface{}), "interface")
	if !reflect.DeepEqual(expectedMap["summary"], actualMap["summary"]) {
		return false
	}

	// find key that holds expected output, e.g. "sip,dport" or "talk_conv"
	var expectedOutputKey string
	for key, _ := range expectedMap {
		if key != "ext_ips" && key != "status" && key != "summary" {
			expectedOutputKey = key
		}
	}

	// find key that holds actual output
	var actualOutputKey string
	for key, _ := range actualMap {
		if key != "ext_ips" && key != "status" && key != "summary" {
			actualOutputKey = key
		}
	}

	// Compare outputs ignoring order (output order of goQuery is non-deterministic, cf. TMI-91)
	expectedOutputs, isSlice := expectedMap[expectedOutputKey].([]interface{})
	if !isSlice {
		return false
	}

	actualOutputs, isSlice := actualMap[actualOutputKey].([]interface{})
	if !isSlice {
		return false
	}

	if len(expectedOutputs) != len(actualOutputs) {
		return false
	}

	expectedOutputSet := make(map[row]struct{})
	for _, output := range expectedOutputs {
		row, isValidRow := newRow(output)
		if !isValidRow {
			return false
		}
		expectedOutputSet[row] = struct{}{}
	}

	for _, output := range actualOutputs {
		row, isValidRow := newRow(output)
		if !isValidRow {
			return false
		}
		if _, exists := expectedOutputSet[row]; !exists {
			return false
		}
	}

	return true
}

// Helper struct that contains a superset of the columns present in any goQuery output.
// We use this as a key for a go map.
// Note that we use float64 for all numeric columns because this struct is filled with data
// from JSON which only supports floats for numberic data.
type row struct {
	bytes, bytes_rcvd, bytes_sent       float64
	bytes_percent                       float64
	dip                                 string
	dport                               string
	iface                               string
	packets, packets_rcvd, packets_sent float64
	packets_percent                     float64
	proto                               string
	sip                                 string
	time                                string
}

// Given an interface{} resulting from a call to json.Unmarshal(), tries to construct a row structure.
func newRow(input interface{}) (result row, ok bool) {
	ok = true

	// map[string]interface{} corresponds to objects in JSON. All rows are output as JSON objects
	// by goQuery.
	inputMap, isMap := input.(map[string]interface{})
	if !isMap {
		return row{}, false
	}

	// Tries to extract a float from inputMap[name]. If inputMap has no element with key name,
	// that is no reason for an error: The goQuery output might not have contained such an element.
	// On the other hand, if inputMap[name] has the wrong dynamic type, something is wrong and we set
	// ok to false.
	extractFloat64 := func(name string, dst *float64) {
		elem, present := inputMap[name]
		if !present {
			return
		}
		elemFloat, isFloat := elem.(float64)
		if !isFloat {
			ok = false
			return
		}
		*dst = elemFloat
	}

	// Like extractFloat64, but for strings
	extractString := func(name string, dst *string) {
		elem, present := inputMap[name]
		if !present {
			return
		}
		elemString, isString := elem.(string)
		if !isString {
			ok = false
			return
		}
		*dst = elemString
	}

	// Construct row and return
	extractFloat64("bytes", &result.bytes)
	extractFloat64("bytes_rcvd", &result.bytes_rcvd)
	extractFloat64("bytes_sent", &result.bytes_sent)
	extractFloat64("bytes_percent", &result.bytes_percent)
	extractString("dip", &result.dip)
	extractString("dport", &result.dport)
	extractString("iface", &result.iface)
	extractFloat64("packets", &result.packets)
	extractFloat64("packets_rcvd", &result.packets_rcvd)
	extractFloat64("packets_sent", &result.packets_sent)
	extractFloat64("packets_percent", &result.packets_percent)
	extractString("proto", &result.proto)
	extractString("sip", &result.sip)
	extractString("time", &result.time)
	return
}
