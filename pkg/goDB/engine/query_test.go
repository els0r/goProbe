package engine

import (
	"bytes"
	"context"
	"testing"

	"github.com/els0r/goProbe/pkg/query"
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
		opts  []query.Option
	}{
		{"eth1 talk_conv with condition - json output", "eth1", "talk_conv", []query.Option{query.WithDBPath(TestDB), query.WithFirst("-30000d"), query.WithCondition("dport < 100 & dport > 100"), query.WithFormat("json")}},
		// border case:
		// the value of the -l parameter forces us to consider the day 1456358400,
		// but day 1456358400 contains no blocks with timestamp < 1456428875
		// (= 1456428575 + DB_WRITEOUT_INTERVAL).
		{"eth1 raw - json output", "eth1", "raw", []query.Option{query.WithDBPath(TestDB), query.WithFirst("-30000d"), query.WithLast("1456428575"), query.WithFormat("json")}},
	}

	// run table-driven test
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// create args
			a := query.NewArgs(test.query, test.iface, test.opts...)

			// prepare query
			stmt, err := a.Prepare()
			if err != nil {
				t.Fatalf("prepare query: %s; args: %s", err, a)
			}

			// capture output in buffer
			var buf = &bytes.Buffer{}
			stmt.Output = buf

			// execute query
			_, err = NewQueryRunner().Run(context.Background(), stmt)
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
		opts  []query.Option
	}{
		{
			"time query on eth1 - json output",
			"eth1",
			"time",
			[]query.Option{query.WithDirectionSum(), query.WithDBPath(TestDB), query.WithFirst("1456428000"), query.WithLast("1456473000"), query.WithNumResults(query.MaxResults), query.WithFormat("json")},
		},
	}

	// run table-driven test
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// append output capture (to dev null, since this test only checks if the query
			// can be exectued
			test.opts = append(test.opts, query.WithOutput("/dev/null"))

			// create args
			a := query.NewArgs(test.query, test.iface, test.opts...)

			// prepare query
			stmt, err := a.Prepare()
			if err != nil {
				t.Fatalf("prepare query: %s", err)
			}

			// execute query
			_, err = NewQueryRunner().Run(context.Background(), stmt)
			if err != nil {
				t.Fatalf("execute query: %s", err)
			}
		})
	}
}
