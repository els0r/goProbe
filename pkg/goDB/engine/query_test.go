package engine

import (
	"context"
	"io"
	"testing"

	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/stretchr/testify/require"
)

var (
	TestDB = "./testdb"
)

// Check that goQuery correctly handles the case where data is missing.
func TestDataMissing(t *testing.T) {

	var tests = []struct {
		name  string
		iface string
		query string
		opts  []query.Option
	}{
		{"eth1 raw - json output", "eth1", "raw", []query.Option{query.WithFirst("-30001d"), query.WithLast("-30000d"), query.WithFormat(types.FormatJSON)}},
	}

	// run table-driven test
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// create args
			a := query.NewArgs(test.query, test.iface, test.opts...)

			// execute query
			res, err := NewQueryRunner(TestDB).Run(context.Background(), a)
			if err != nil {
				t.Fatalf("execute query: %s", err)
			}

			result := res
			if result.Status.Code != types.StatusMissingData {
				t.Fatalf("unexpected status %q: %s", result.Status.Code, result.Status.Message)
			}
		})
	}
}

// Check that goQuery correctly handles the case where there is no output.
func TestEmptyOutput(t *testing.T) {

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
			[]query.Option{query.WithDirectionSum(), query.WithFirst("1456358400"), query.WithLast("1456473000"), query.WithCondition("sip = 255.255.255.255"), query.WithNumResults(query.MaxResults), query.WithFormat(types.FormatJSON)},
		},
	}

	// run table-driven test
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// create args
			a := query.NewArgs(test.query, test.iface, test.opts...)

			// execute query
			res, err := NewQueryRunner(TestDB).Run(context.Background(), a)
			if err != nil {
				t.Fatalf("execute query: %s", err)
			}

			result := res
			if result.Status.Code != types.StatusEmpty {
				t.Fatalf("unexpected status %q: %s", result.Status.Code, result.Status.Message)
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
			[]query.Option{query.WithDirectionSum(), query.WithFirst("1456428000"), query.WithLast("1456473000"), query.WithNumResults(query.MaxResults), query.WithFormat(types.FormatJSON)},
		},
	}

	// run table-driven test
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// append output capture (to dev null, since this test only checks if the query
			// can be exectued

			// create args
			a := query.NewArgs(test.query, test.iface, test.opts...).AddOutputs(io.Discard)

			// execute query
			res, err := NewQueryRunner(TestDB).Run(context.Background(), a)
			if err != nil {
				t.Fatalf("execute query: %s", err)
			}

			if len(res.Rows) == 0 {
				t.Fatal("query result unexpectedly empty")
			}
		})
	}
}

type MockInterfaceLister struct {
	interfaces []string
}

func NewMockInterfaceLister(interfaces []string) *MockInterfaceLister {
	return &MockInterfaceLister{interfaces: interfaces}
}

func (mil MockInterfaceLister) ListInterfaces() ([]string, error) {
	return mil.interfaces, nil
}

type filteringTestDefinition struct {
	name     string
	argument string
	ifaces   []string
	expected []string
}

func TestCommaSeparatedInterfaceFiltering(t *testing.T) {
	var tests = []filteringTestDefinition{
		{
			"selected interfaces are returned",
			"eth0, eth2",
			[]string{"eth0", "wlan0", "eth2"},
			[]string{"eth0", "eth2"},
		},
		{
			"nonexistent interface is ignored",
			"eth0, eth2, notexistent",
			[]string{"eth0", "wlan0", "eth2"},
			[]string{"eth0", "eth2"},
		},
		{
			"any arguments returns all ignored",
			"any",
			[]string{"eth0", "wlan0", "eth2"},
			[]string{"eth0", "wlan0", "eth2"},
		},
		{
			"all but negaded interface is returned",
			"any,!eth2,!t1",
			[]string{"eth0", "wlan0", "t1", "t2", "eth2"},
			[]string{"eth0", "wlan0", "t2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lister := NewMockInterfaceLister(test.ifaces)
			actual, err := parseIfaceListWithCommaSeparatedString(lister, test.argument)
			if err == nil {
				require.EqualValues(t, test.expected, actual)
			}
		})
	}
}

func TestRegExpInterfaceFiltering(t *testing.T) {
	var tests = []filteringTestDefinition{
		{
			"precisely 1 interface is returned",
			"/eth0/",
			[]string{"eth0", "wlan0", "eth2"},
			[]string{"eth0"},
		},
		{
			"eth prefixed interfaces are selected with correct number range",
			"/eth[0-2]/",
			[]string{"eth0", "wlan0", "eth2", "eth3"},
			[]string{"eth0", "eth2"},
		},
		{
			"using regep or for smaller expression",
			"/eth[0-2]|wlan0|t4/",
			[]string{"eth0", "wlan0", "eth2", "t4", "ignored"},
			[]string{"eth0", "wlan0", "eth2", "t4"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lister := NewMockInterfaceLister(test.ifaces)
			actual, err := parseIfaceListWithRegex(lister, test.argument)
			if err == nil {
				require.EqualValues(t, test.expected, actual)
			}
			require.Nil(t, err, "No error expected for given input.")
		})
	}
}

func TestDetectRegExpArgument(t *testing.T) {

	var tests = []struct {
		arg    string
		result bool
	}{
		{
			"",
			false,
		},
		{
			"/eth/",
			true,
		},
		{
			"//eth//",
			true,
		},
		{
			"eth0",
			false,
		},
		{
			"//",
			false,
		},
		{
			"/eth0",
			false,
		},
		{
			"eth0",
			false,
		}, {
			"eth0/",
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.arg, func(t *testing.T) {
			actual := types.IsIfaceArgumentRegExp(test.arg)
			require.EqualValues(t, actual, test.result)
		})
	}
}
