package query

import (
	"errors"
	"testing"

	"github.com/els0r/goProbe/pkg/goDB/conditions/node"
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
			&ArgsError{
				Field:   "field",
				Type:    UnknownErrorType,
				Message: "an error occurred",
				err:     nil,
			},
			[]byte(`{"field":"field","type":"UnknownError","message":"an error occurred"}`),
		},
		{"simple error, underlying error set",
			&ArgsError{
				Field:   "field",
				Type:    UnknownErrorType,
				Message: "an error occurred",
				err:     errors.New("an error"),
			},
			[]byte(`{"field":"field","type":"UnknownError","message":"an error occurred","error":"an error"}`),
		},
		{"detailed parsing error",
			&ArgsError{
				Field:   "condition",
				Type:    ParseErrorType,
				Message: "parsing failed",
				err:     &node.ParseError{},
			},
			[]byte(`{"field":"condition","type":"ParseError","message":"parsing failed","error":{"tokens":null,"pos":0,"description":""}}`),
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
		{"empty", &Args{}, &ArgsError{
			Field:   "query",
			Type:    ParseErrorType,
			Message: "invalid query type",
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_, err := test.input.Prepare()
			if test.err == nil {
				require.Nil(t, err)
				return
			}
			require.Equal(t, test.err, err)
		})
	}
}
