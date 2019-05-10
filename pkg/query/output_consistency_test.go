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

package query

import (
	"bytes"
	"encoding/json"
	"errors"
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
	outputConsistencyDir = "./output_consistency"
	argsSuffix           = ".args.json"
	correctOutputSuffix  = ".correctOutput.json"
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
	t.Parallel()

	CheckDBExists(TestDB)

	testCases, err := testCases()
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range testCases {

		// Read test inputs
		argumentFile := path.Join(outputConsistencyDir, testCase+argsSuffix)
		expectedOutputFile := path.Join(outputConsistencyDir, testCase+correctOutputSuffix)

		// Read arguments
		argumentsJSON, err := ioutil.ReadFile(argumentFile)
		if err != nil {
			t.Fatalf("Could not read argument file %s. Error: %s", argumentFile, err)
		}

		var arguments []Args
		err = json.Unmarshal(argumentsJSON, &arguments)
		if err != nil {
			t.Fatalf("Could not decode argument file %s. Error: %s", argumentFile, err)
		}

		// Read expected output
		expectedOutputJSON, err := ioutil.ReadFile(expectedOutputFile)
		if err != nil {
			t.Fatalf("Could not read expected output file %s. Error: %s", expectedOutputFile, err)
		}
		var expectedOutput interface{}
		err = json.Unmarshal(expectedOutputJSON, &expectedOutput)
		if err != nil {
			t.Fatalf("Could not decode expected output file %s. Error: %s", expectedOutputFile, err)
		}

		var buf = &bytes.Buffer{}
		var stmt *Statement
		for i, args := range arguments {
			buf.Reset()

			// prepare query
			stmt, err = args.Prepare(buf)
			if err != nil {
				t.Fatalf("[%d] failed to prepare query: %s", i, err)
			}

			// run query
			err = stmt.Execute()
			if err != nil {
				t.Fatalf("[%d] failed to run query: %s", i, err)
			}

			actualOutputJSON := buf.Bytes()

			var actualOutput interface{}
			err = json.Unmarshal(actualOutputJSON, &actualOutput)
			if err != nil {
				t.Fatalf("[%d] failed to decode JSON output: %s\n%s", i, err, string(actualOutputJSON))
			}
			var match bool
			match, err = outputMatches(expectedOutput, actualOutput)
			if err != nil || !match {
				t.Logf("[%d] arguments: from %s\n%s", i, argumentFile, args.String())
				//				mi, _ := json.MarshalIndent(expectedOutput, "", " ")
				//				t.Fatalf("[%d] output from testcase %s doesn't match correct output.\nWant: %s", i, testCase, string(mi))
				t.Fatalf("[%d] output from testcase %s doesn't match correct output: %s", i, testCase, err)
			}
		}
	}
}

func testCases() (testCases []string, err error) {
	// Open file descriptor for outputConsistencyDir
	fd, err := os.Open(outputConsistencyDir)
	if err != nil {
		err = fmt.Errorf("Could not open directory %v: %v", outputConsistencyDir, err)
		return
	}
	// Get a list of ALL files in the directory, that's what the -1 is for.
	fileInfos, err := fd.Readdir(-1)
	if err != nil {
		err = fmt.Errorf("Could not enumerate directory %v: %v", outputConsistencyDir, err)
		return
	}

	// Map to keep track of whether (for each testcase) we have seen a correctOutput file AND an args file.
	seen := make(map[string]struct {
		correctOutput bool
		args          bool
	})

	// Loop over all entries of outputConsistencyDir and populate seen map.
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}

		name := fileInfo.Name()
		if strings.HasSuffix(name, argsSuffix) {
			key := name[:len(name)-len(argsSuffix)]
			seenElem := seen[key]
			seenElem.args = true
			seen[key] = seenElem
		} else if strings.HasSuffix(name, correctOutputSuffix) {
			key := name[:len(name)-len(correctOutputSuffix)]
			seenElem := seen[key]
			seenElem.correctOutput = true
			seen[key] = seenElem
		}
	}

	// Check validity condition, i.e. whether for each testcase we have both files.
	for testName, seenFiles := range seen {
		if !seenFiles.args {
			return nil, fmt.Errorf("File %v%v is missing", testName, argsSuffix)
		}
		if !seenFiles.correctOutput {
			return nil, fmt.Errorf("File %v%v is missing", testName, correctOutputSuffix)
		}
	}

	// If we get here, the validity condition is satisfied; we return the list of test cases.
	for testName := range seen {
		testCases = append(testCases, testName)
	}
	return
}

// Check whether the actual output and the expected output (both as interface{}s
// unmarshalled from JSON) match.
func outputMatches(expected, actual interface{}) (ok bool, err error) {
	expectedMap, isMap := expected.(map[string]interface{})
	if !isMap {
		return false, errors.New("expected output is not a map")
	}

	actualMap, isMap := actual.(map[string]interface{})
	if !isMap {
		return false, errors.New("actual output is not a map")
	}

	if len(actualMap) != 4 || len(expectedMap) != 4 {
		return false, errors.New("loaded maps have incorrect number of sections")
	}

	// ext_ips: we only check that these are present, but don't compare
	// them, because they vary from host to host.
	if _, isSlice := expectedMap["ext_ips"].([]interface{}); !isSlice {
		return false, errors.New("in expected: didn't find external IPs section")
	}

	if _, isSlice := actualMap["ext_ips"].([]interface{}); !isSlice {
		return false, errors.New("in actual: didn't find external IPs section")
	}

	// status
	if !reflect.DeepEqual(expectedMap["status"], actualMap["status"]) {
		return false, fmt.Errorf("status doesn't match. Have: %s; Want: %s", fmt.Sprint(actualMap["status"]), fmt.Sprint(expectedMap["status"]))
	}

	// summary
	// there are multiple ways to write the same interface list
	delete(expectedMap["summary"].(map[string]interface{}), "interface")
	delete(actualMap["summary"].(map[string]interface{}), "interface")
	if !reflect.DeepEqual(expectedMap["summary"], actualMap["summary"]) {
		return false, fmt.Errorf("summary doesn't match.\nHave:\n%s;\nWant:\n%s", fmt.Sprint(actualMap["summary"]), fmt.Sprint(expectedMap["summary"]))
	}

	// find key that holds expected output, e.g. "sip,dport" or "talk_conv"
	var expectedOutputKey string
	for key := range expectedMap {
		if key != "ext_ips" && key != "status" && key != "summary" {
			expectedOutputKey = key
		}
	}

	// find key that holds actual output
	var actualOutputKey string
	for key := range actualMap {
		if key != "ext_ips" && key != "status" && key != "summary" {
			actualOutputKey = key
		}
	}

	// Compare outputs ignoring order (output order of goQuery is non-deterministic, cf. TMI-91)
	expectedOutputs, isSlice := expectedMap[expectedOutputKey].([]interface{})
	if !isSlice {
		return false, fmt.Errorf("expected output is not a slice")
	}

	actualOutputs, isSlice := actualMap[actualOutputKey].([]interface{})
	if !isSlice {
		return false, fmt.Errorf("actual output is not a slice")
	}

	if len(expectedOutputs) != len(actualOutputs) {
		return false, fmt.Errorf("number of entries mismatch. len(expected)=%d; len(actual)=%d", len(expectedOutputs), len(actualOutputs))
	}

	expectedOutputSet := make(map[row]struct{})
	for i, output := range expectedOutputs {
		row, isValidRow := newRow(output)
		if !isValidRow {
			return false, fmt.Errorf("expected: invalid row at index %d", i)
		}
		expectedOutputSet[row] = struct{}{}
	}

	for i, output := range actualOutputs {
		row, isValidRow := newRow(output)
		if !isValidRow {
			return false, fmt.Errorf("actual: invalid row at index %d", i)
		}
		if _, exists := expectedOutputSet[row]; !exists {
			return false, fmt.Errorf("row of actual doesn't exist in expected at index %d", i)
		}
	}

	return true, nil
}

// Helper struct that contains a superset of the columns present in any goQuery output.
// We use this as a key for a go map.
// Note that we use float64 for all numeric columns because this struct is filled with data
// from JSON which only supports floats for numberic data.
type row struct {
	bytes, bytesRcvd, bytesSent       float64
	bytesPercent                      float64
	dip                               string
	dport                             string
	iface                             string
	packets, packetsRcvd, packetsSent float64
	packetsPercent                    float64
	proto                             string
	sip                               string
	time                              string
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
	extractFloat64("bytesRcvd", &result.bytesRcvd)
	extractFloat64("bytesSent", &result.bytesSent)
	extractFloat64("bytesPercent", &result.bytesPercent)
	extractString("dip", &result.dip)
	extractString("dport", &result.dport)
	extractString("iface", &result.iface)
	extractFloat64("packets", &result.packets)
	extractFloat64("packetsRcvd", &result.packetsRcvd)
	extractFloat64("packetsSent", &result.packetsSent)
	extractFloat64("packetsPercent", &result.packetsPercent)
	extractString("proto", &result.proto)
	extractString("sip", &result.sip)
	extractString("time", &result.time)
	return
}
