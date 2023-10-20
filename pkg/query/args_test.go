package query

import (
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
)

func TestMarshalArgsError(t *testing.T) {
	var tests = []struct {
		name     string
		input    *ArgsError
		expected []byte
	}{
		{"nil", nil, []byte("null")},
		{"simple error, underlying error nil",
			newArgsError(
				"field",
				"an error occurred",
				nil,
			),
			[]byte(`{"field":"field","message":"an error occurred"}`),
		},
		{"simple error, underlying error set",
			newArgsError(
				"field",
				"an error occurred",
				errors.New("an error"),
			),
			[]byte(`{"field":"field","type":"*errors.errorString","message":"an error occurred","error":"an error"}`),
		},
		{"detailed parsing error",
			newArgsError(
				"condition",
				"parsing failed",
				types.NewParseError([]string{"sipl", "=", "192.168.1.1"}, 0, " ", "Expected attribute"),
			),
			[]byte(`{"field":"condition","type":"*types.ParseError","message":"parsing failed","error":{"tokens":["sipl","=","192.168.1.1"],"pos":0,"description":"Expected attribute"}}`),
		},
		{"unsupported error",
			newArgsError(
				"sort_by",
				"wrong sort by",
				types.NewUnsupportedError("biscuits", []string{"a", "b"}),
			),
			[]byte(`{"field":"sort_by","type":"*types.UnsupportedError","message":"wrong sort by","error":{"val":"biscuits","valid":["a","b"]}}`),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			b, err := jsoniter.Marshal(test.input)
			require.Nil(t, err)
			require.Equal(t, string(test.expected), string(b))
		})
	}
}

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
				Query: "sip,time", Format: "json", Last: "-7d",
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

			actual, ok := err.(*ArgsError)
			require.Truef(t, ok, "expected error to be of type %T", &ArgsError{})

			// individually compare the struct fields. Why the detour? So we don't have
			// to re-create (and test) errors that are caught by other packages (such as
			// parsing errors)
			require.Equal(t, test.err.Field, actual.Field)
			require.Equal(t, test.err.Type, actual.Type)
			require.Equal(t, test.err.Message, actual.Message)
		})
	}
}
