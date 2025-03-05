package client

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestEventStream(t *testing.T) {
	var tests = []struct {
		body          io.Reader
		line          []byte
		expectedEvent *event
	}{
		{
			body: strings.NewReader(`
event: partialResult
data: hello
`),
			expectedEvent: &event{
				streamType: api.StreamEventPartialResult,
				data:       []byte("hello"),
			},
		},
		{
			body: strings.NewReader(`


event: finalResult
data: hello
`),
			expectedEvent: &event{
				streamType: api.StreamEventFinalResult,
				data:       []byte("hello"),
			},
		},
		{
			body: strings.NewReader(`


event: queryError
data: there was an error
`),
			expectedEvent: &event{
				streamType: api.StreamEventQueryError,
				data:       []byte("there was an error"),
			},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			r := bufio.NewReader(test.body)
			actual, err := readEvent(r)
			require.Nil(t, err)
			require.Equal(t, test.expectedEvent, actual)
		})
	}
}
