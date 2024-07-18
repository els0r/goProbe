package query

import (
	"errors"
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
		err   *DetailError
	}{
		{"empty", &Args{},
			&DetailError{},
		},
		{"unsupported format", &Args{Query: "sip", Format: "exe"},
			&DetailError{},
		},
		{"wrong sort by", &Args{Query: "sip", Format: types.FormatJSON, SortBy: "biscuits"},
			&DetailError{},
		},
		{"empty sort by, invalid time",
			&Args{Query: "sip,time", Format: types.FormatJSON, First: "10:"},
			&DetailError{},
		},
		{"dns resolution, wrong timeout",
			&Args{
				Query: "sip,time", Format: types.FormatJSON, First: "-7d",
				DNSResolution: DNSResolution{
					Enabled: true,
					Timeout: -1,
				},
			},
			&DetailError{},
		},
		{"dns resolution, wrong number of rows",
			&Args{
				Query: "sip,time", Format: types.FormatJSON, First: "-7d",
				DNSResolution: DNSResolution{
					Enabled: true,
					Timeout: 20 * time.Second,
					MaxRows: -1,
				},
			},
			&DetailError{},
		},
		{"incorrect condition",
			&Args{
				Query: "sip,time", Format: types.FormatJSON, First: "-7d",
				Condition: "dipl = 0",
			},
			&DetailError{},
		},
		{"incorrect mem percentage",
			&Args{
				Query: "sip,time", Format: types.FormatJSON, First: "-7d",
			},
			&DetailError{},
		},
		{"wrong number of results",
			&Args{
				Query: "sip,time", Format: types.FormatJSON, First: "-7d",
				MaxMemPct: 20,
			},
			&DetailError{},
		},
		{"invalid live mode",
			&Args{
				Query: "sip,time", Format: types.FormatJSON, Last: "-7d",
				MaxMemPct: 20, NumResults: 20,
				Live: true,
			},
			&DetailError{},
		},
		{"valid query args",
			&Args{
				Ifaces: "eth0",
				Query:  "sip,time", Format: types.FormatJSON, Last: "-7d",
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

			ok := errors.As(err, &test.err)
			require.Truef(t, ok, "expected error to be of type %T", &DetailError{})
		})
	}
}

func TestSelector(t *testing.T) {
	var tests = []struct {
		name     string
		input    *Args
		selector types.LabelSelector
		err      *DetailError
	}{
		{
			name: "empty interface",
			input: &Args{
				Query: "sip", Format: types.FormatJSON, Last: "-7d",
				MaxMemPct: 20, NumResults: 20,
				outputs: []io.Writer{os.Stdout, os.Stderr},
			},
			selector: types.LabelSelector{},
			err:      &DetailError{},
		},
		{
			name: "single interface",
			input: &Args{
				Ifaces: "eth0",
				Query:  "sip", Format: types.FormatJSON, Last: "-7d",
				MaxMemPct: 20, NumResults: 20,
				outputs: []io.Writer{os.Stdout, os.Stderr},
			},
			selector: types.LabelSelector{},
		},
		{
			name: "two interfaces",
			input: &Args{
				Ifaces: "eth0,eth2",
				Query:  "sip", Format: types.FormatJSON, Last: "-7d",
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
				Query:  "sip", Format: types.FormatJSON, Last: "-7d",
				MaxMemPct: 20, NumResults: 20,
				outputs: []io.Writer{os.Stdout, os.Stderr},
			},
			err: &DetailError{},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			stmt, err := test.input.Prepare()
			if test.err != nil {
				require.ErrorAs(t, err, &test.err)

				t.Log(err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, test.selector, stmt.LabelSelector)
		})
	}
}
