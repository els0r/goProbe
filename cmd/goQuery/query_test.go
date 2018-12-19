/////////////////////////////////////////////////////////////////////////////////
//
// query_test.go
//
// Written by Lorenz Breidenbach lob@open.ch, August 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

var emptyOutputArgs = [][]string{
	{"-i", "eth1", "-d", SMALL_GODB, "-f", "-30000d", "-c", "dport < 100 & dport > 100", "-e", "json", "talk_conv"},
	// border case:
	// the value of the -l parameter forces us to consider the day 1456358400,
	// but day 1456358400 contains no blocks with timestamp < 1456428875
	// (= 1456428575 + DB_WRITEOUT_INTERVAL).
	{"-i", "eth1", "-d", SMALL_GODB, "-f", "-30000d", "-l", "1456428575", "-e", "json", "raw"},
}

// Check that goQuery correctly handles the case where there is no output.
func TestEmptyOutput(t *testing.T) {
	t.Parallel()

	checkDbExists(t, SMALL_GODB)

	for _, args := range emptyOutputArgs {
		cmd := callMain(args...)
		actualOutputJson, err := cmd.Output()
		if err != nil {
			t.Fatalf("Error running goQuery")
		}

		// We are running our real main() inside the test executable's main(). The latter always prints PASS\n
		// if there were no errors. This makes the JSON parser unhappy, so we remove it from the output.
		actualOutputJson = actualOutputJson[:len(actualOutputJson)-len("PASS\n")]

		var actualOutput map[string]string
		err = json.Unmarshal(actualOutputJson, &actualOutput)
		if err != nil {
			t.Log(actualOutputJson)
			t.Log(args)
			t.Fatalf("Failed to parse output as JSON: %s", err)
		}
		if actualOutput["status"] != "empty" || actualOutput["statusMessage"] != ERROR_NORESULTS {
			t.Fatalf("Unexpected output: %v", actualOutput)
		}
	}
}

var dnsArgs = []string{
	"-i", "eth1", "-d", SMALL_GODB, "-f", "-30000d", "-c", "sip = 1b5ec0b902e689122ededcfac01ee69ed3c8422c.open.ch", "talk_conv",
}

// Checks whether name resolution for conditionals is attempted.
func TestDNS(t *testing.T) {
	t.Parallel()

	checkDbExists(t, SMALL_GODB)

	output, err := callMain(dnsArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("Error running goQuery")
	}
	if !strings.Contains(string(output), "no such host") {
		t.Fatalf("Expected to get 'no such host' error.")
	}
}
