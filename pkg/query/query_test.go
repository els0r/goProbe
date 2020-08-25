package query

import (
	"bytes"
	"testing"

	jsoniter "github.com/json-iterator/go"
)

var (
	TestDB = "./testdb"
)

// Check that goQuery correctly handles the case where there is no output.
func TestEmptyOutput(t *testing.T) {

	var tests = []struct {
		name  string
		iface string
		query string
		opts  []Option
	}{
		{"eth1 talk_conv with condition - json output", "eth1", "talk_conv", []Option{WithDBPath(TestDB), WithFirst("-30000d"), WithCondition("dport < 100 & dport > 100"), WithFormat("json")}},
		// border case:
		// the value of the -l parameter forces us to consider the day 1456358400,
		// but day 1456358400 contains no blocks with timestamp < 1456428875
		// (= 1456428575 + DB_WRITEOUT_INTERVAL).
		{"eth1 raw - json output", "eth1", "raw", []Option{WithDBPath(TestDB), WithFirst("-30000d"), WithLast("1456428575"), WithFormat("json")}},
	}

	// run table-driven test
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// create args
			a := NewArgs(test.query, test.iface, test.opts...)

			// prepare query
			stmt, err := a.Prepare()
			if err != nil {
				t.Fatalf("prepare query: %s; args: %s", err, a)
			}

			// capture output in buffer
			var buf = &bytes.Buffer{}
			stmt.Output = buf

			// execute query
			err = stmt.Execute()
			if err != nil {
				t.Fatalf("execute query: %s", err)
			}

			actualOutputJSON := buf.Bytes()

			var actualOutput map[string]string
			err = jsoniter.Unmarshal(actualOutputJSON, &actualOutput)
			if err != nil {
				t.Log(string(actualOutputJSON))
				t.Log(a)
				t.Fatalf("failed to parse output as JSON: %s", err)
			}
			if actualOutput["status"] != "empty" || actualOutput["statusMessage"] != errorNoResults.Error() {
				t.Fatalf("unexpected output: %v", actualOutput)
			}
		})
	}
}

func TestSimpleQuery(t *testing.T) {

	// create args
	var tests = []struct {
		name  string
		iface string
		query string
		opts  []Option
	}{
		{
			"time query on eth1 - json output",
			"eth1",
			"time",
			[]Option{WithDirectionSum(), WithDBPath(TestDB), WithFirst("1456428000"), WithLast("1456473000"), WithNumResults(MaxResults), WithFormat("json")},
		},
	}

	// run table-driven test
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// append output capture (to dev null, since this test only checks if the query
			// can be exectued
			test.opts = append(test.opts, WithOutput("/dev/null"))

			// create args
			a := NewArgs(test.query, test.iface, test.opts...)

			// prepare query
			stmt, err := a.Prepare()
			if err != nil {
				t.Fatalf("prepare query: %s", err)
			}

			// execute query
			err = stmt.Execute()
			if err != nil {
				t.Fatalf("execute query: %s", err)
			}
		})
	}
}
