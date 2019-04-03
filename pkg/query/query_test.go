package query

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

var (
	TestDB = "./testdb"
)

// Check that goQuery correctly handles the case where there is no output.
func TestEmptyOutput(test *testing.T) {

	var t = &testWrapper{
		t: test,
		tests: []queryTest{
			{"eth1", "talk_conv", []Option{WithDBPath(TestDB), WithFirst("-30000d"), WithCondition("dport < 100 & dport > 100"), WithFormat("json")}},
			// border case:
			// the value of the -l parameter forces us to consider the day 1456358400,
			// but day 1456358400 contains no blocks with timestamp < 1456428875
			// (= 1456428575 + DB_WRITEOUT_INTERVAL).
			{"eth1", "raw", []Option{WithDBPath(TestDB), WithFirst("-30000d"), WithLast("1456428575"), WithFormat("json")}},
		},
	}

	// run table-driven test
	t.iterate(func() {

		// create args
		a := NewArgs(t.cur.query, t.cur.ifaces, t.cur.options...)

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

		actualOutputJson := buf.Bytes()

		var actualOutput map[string]string
		err = json.Unmarshal(actualOutputJson, &actualOutput)
		if err != nil {
			t.t.Log(string(actualOutputJson))
			t.t.Log(a)
			t.Fatalf("failed to parse output as JSON: %s", err)
		}
		if actualOutput["status"] != "empty" || actualOutput["statusMessage"] != errorNoResults.Error() {
			t.Fatalf("unexpected output: %v", actualOutput)
		}
	})
}

func TestSimpleQuery(test *testing.T) {

	// create args
	var t = &testWrapper{
		t: test,
		tests: []queryTest{
			{
				"eth1",
				"time",
				[]Option{WithDirectionSum(), WithDBPath(TestDB), WithFirst("1456428000"), WithLast("1456473000"), WithNumResults(MaxResults), WithFormat("json")},
			},
		},
	}

	// run table-driven test
	t.iterate(func() {

		// append output capture (to dev null, since this test only checks if the query
		// can be exectued
		t.cur.options = append(t.cur.options, WithOutput("/dev/null"))

		// create args
		a := NewArgs(t.cur.query, t.cur.ifaces, t.cur.options...)

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

type queryTest struct {
	ifaces  string
	query   string
	options []Option
}

type testWrapper struct {
	t     *testing.T
	i     int // for table driven tests
	cur   queryTest
	tests []queryTest
}

func (t *testWrapper) iterate(f func()) {
	for i, args := range t.tests {
		t.cur = args
		t.i = i
		f()
	}
}

func (t *testWrapper) Fatalf(fmtStr string, args ...interface{}) {
	str := fmt.Sprintf("[%d] ", t.i)
	t.t.Fatalf(str+fmtStr, args...)
}
