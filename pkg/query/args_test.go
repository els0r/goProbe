package query

import (
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/els0r/goProbe/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestPrepareArgs(t *testing.T) {
	var tests = []struct {
		name  string
		input *Args
		err   *ArgsError
	}{
		{"empty", &Args{},
			&ArgsError{
				Field:   "query",
				Message: invalidQueryTypeMsg,
				Type:    fmt.Sprintf("%T", &types.ParseError{}),
			},
		},
		{"unsupported format", &Args{Query: "sip", Format: "exe"},
			&ArgsError{
				Field:   "format",
				Message: invalidFormatMsg,
				Type:    fmt.Sprintf("%T", &types.UnsupportedError{}),
			},
		},
		{"wrong sort by", &Args{Query: "sip", Format: "json", SortBy: "biscuits"},
			&ArgsError{
				Field:   "sort_by",
				Message: invalidSortByMsg,
				Type:    fmt.Sprintf("%T", &types.UnsupportedError{}),
			},
		},
		{"empty sort by, invalid time",
			&Args{Query: "sip,time", Format: "json", First: "10:"},
			&ArgsError{
				Field:   "first/last",
				Message: invalidTimeRangeMsg,
				Type:    "*fmt.wrapErrors",
			},
		},
		{"dns resolution, wrong timeout",
			&Args{
				Query: "sip,time", Format: "json", First: "-7d",
				DNSResolution: DNSResolution{
					Enabled: true,
					Timeout: -1,
				},
			},
			&ArgsError{
				Field:   "dns_resolution.timeout",
				Message: invalidDNSResolutionTimeoutMsg,
				Type:    fmt.Sprintf("%T", &types.MinBoundsError{}),
			},
		},
		{"dns resolution, wrong number of rows",
			&Args{
				Query: "sip,time", Format: "json", First: "-7d",
				DNSResolution: DNSResolution{
					Enabled: true,
					Timeout: 20 * time.Second,
					MaxRows: -1,
				},
			},
			&ArgsError{
				Field:   "dns_resolution.max_rows",
				Message: invalidDNSResolutionRowsMsg,
				Type:    fmt.Sprintf("%T", &types.MinBoundsError{}),
			},
		},
		{"incorrect condition",
			&Args{
				Query: "sip,time", Format: "json", First: "-7d",
				Condition: "dipl = 0",
			},
			&ArgsError{
				Field:   "condition",
				Message: invalidConditionMsg,
				Type:    fmt.Sprintf("%T", &types.ParseError{}),
			},
		},
		{"incorrect mem percentage",
			&Args{
				Query: "sip,time", Format: "json", First: "-7d",
			},
			&ArgsError{
				Field:   "max_mem_pct",
				Message: invalidMaxMemPctMsg,
				Type:    fmt.Sprintf("%T", &types.RangeError{}),
			},
		},
		{"wrong number of results",
			&Args{
				Query: "sip,time", Format: "json", First: "-7d",
				MaxMemPct: 20,
			},
			&ArgsError{
				Field:   "num_results",
				Message: invalidRowLimitMsg,
				Type:    fmt.Sprintf("%T", &types.MinBoundsError{}),
			},
		},
		{"invalid live mode",
			&Args{
				Query: "sip,time", Format: "json", Last: "-7d",
				MaxMemPct: 20, NumResults: 20,
				Live: true,
			},
			&ArgsError{
				Field:   "live",
				Message: invalidLiveQueryMsg,
				Type:    "*errors.errorString",
			},
		},
		{"valid query args",
			&Args{
				Ifaces: "eth0",
				Query:  "sip,time", Format: "json", Last: "-7d",
				MaxMemPct: 20, NumResults: 20,
				outputs: []io.Writer{os.Stdout, os.Stderr},
			},
			nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_, err := test.input.Prepare()
			if test.err == nil {
				require.Nil(t, err)
				return
			}

			t.Logf("error:\n%v", err)

			ok := errors.As(err, &test.err)
			require.Truef(t, ok, "expected error to be of type %T", &ArgsError{})

			// individually compare the struct fields. Why the detour? So we don't have
			// to re-create (and test) errors that are caught by other packages (such as
			// parsing errors)
			require.Equal(t, test.err.Field, test.err.Field)
			require.Equal(t, test.err.Type, test.err.Type)
			require.Equal(t, test.err.Message, test.err.Message)
		})
	}
}

func TestSelector(t *testing.T) {
	var tests = []struct {
		name     string
		input    *Args
		selector types.LabelSelector
		err      *ArgsError
	}{
		{
			name: "empty interface",
			input: &Args{
				Query: "sip", Format: "json", Last: "-7d",
				MaxMemPct: 20, NumResults: 20,
				outputs: []io.Writer{os.Stdout, os.Stderr},
			},
			selector: types.LabelSelector{},
			err: &ArgsError{
				Field:   "iface",
				Message: emptyInterfaceMsg,
				Type:    "*types.ParseError",
			},
		},
		{
			name: "single interface",
			input: &Args{
				Ifaces: "eth0",
				Query:  "sip", Format: "json", Last: "-7d",
				MaxMemPct: 20, NumResults: 20,
				outputs: []io.Writer{os.Stdout, os.Stderr},
			},
			selector: types.LabelSelector{},
		},
		{
			name: "two interfaces",
			input: &Args{
				Ifaces: "eth0,eth2",
				Query:  "sip", Format: "json", Last: "-7d",
				MaxMemPct: 20, NumResults: 20,
				outputs: []io.Writer{os.Stdout, os.Stderr},
			},
			selector: types.LabelSelector{
				Iface: true,
			},
		},
		{
			name: "invalid interface name",
			input: &Args{
				Ifaces: "eth0,eth two",
				Query:  "sip", Format: "json", Last: "-7d",
				MaxMemPct: 20, NumResults: 20,
				outputs: []io.Writer{os.Stdout, os.Stderr},
			},
			err: &ArgsError{
				Field:   "iface",
				Message: invalidInterfaceMsg,
				Type:    "*errors.errorString",
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			stmt, err := test.input.Prepare()
			if test.err != nil {
				require.ErrorAs(t, err, &test.err)
				require.Equal(t, test.err.Field, test.err.Field)
				require.Equal(t, test.err.Type, test.err.Type)

				t.Log(err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, test.selector, stmt.LabelSelector)
		})
	}
}
