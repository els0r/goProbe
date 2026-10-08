package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/stretchr/testify/assert"
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

// queryAgainst runs one streaming query against a server answering with handler and
// returns what the client hands its caller
func queryAgainst(t *testing.T, handler http.HandlerFunc) (*results.Result, error) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	noUpdate := func(context.Context, *results.Result) error { return nil }
	noKeepalive := func(context.Context) error { return nil }
	sse := NewSSE(srv.URL, noUpdate, noUpdate, noKeepalive)

	return sse.Query(context.Background(), &query.Args{Query: "sip", Ifaces: "eth0", QueryHosts: "h1"})
}

// problemResponse answers every request with an RFC 9457 problem of the given status and
// detail, as the authorization middleware does before the stream opens
func problemResponse(t *testing.T, status int, detail string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(status)
		_, err := fmt.Fprintf(w, `{"status":%d,"title":%q,"detail":%q}`, status, http.StatusText(status), detail)
		assert.NoError(t, err)
	}
}

// TestSSEClient_Query_SurfacesAuthorizationStatus pins that the streaming client hands its
// caller an error carrying the server's status and detail, whether the server rejected the
// request before the stream opened or reported the failure as a query-error event
func TestSSEClient_Query_SurfacesAuthorizationStatus(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		status  int
		detail  string
	}{
		{
			name: "query-error event",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, err := fmt.Fprint(w, "event: queryError\ndata: {\"status\":403,\"title\":\"Forbidden\",\"detail\":\"no authorized hosts in query\"}\n\n")
				assert.NoError(t, err)
			},
			status: http.StatusForbidden,
			detail: "no authorized hosts in query",
		},
		{
			name:    "unauthenticated before the stream opens",
			handler: problemResponse(t, http.StatusUnauthorized, "unauthenticated"),
			status:  http.StatusUnauthorized,
			detail:  "unauthenticated",
		},
		{
			name:    "forbidden before the stream opens",
			handler: problemResponse(t, http.StatusForbidden, "forbidden"),
			status:  http.StatusForbidden,
			detail:  "forbidden",
		},
		{
			name:    "authorization unavailable before the stream opens",
			handler: problemResponse(t, http.StatusServiceUnavailable, "authorization unavailable"),
			status:  http.StatusServiceUnavailable,
			detail:  "authorization unavailable",
		},
		{
			name: "non-problem body before the stream opens",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusBadGateway)
				_, err := fmt.Fprintln(w, "upstream gone")
				assert.NoError(t, err)
			},
			status: http.StatusBadGateway,
			detail: "502 Bad Gateway: upstream gone",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := queryAgainst(t, tt.handler)

			require.Error(t, err)
			assert.Nil(t, res)
			var statusErr huma.StatusError
			require.ErrorAs(t, err, &statusErr, "error %q carries no status", err)
			assert.Equal(t, tt.status, statusErr.GetStatus())
			assert.Equal(t, tt.detail, statusErr.Error())
		})
	}
}

// TestSSEClient_Query_TruncatesLongErrorBody pins that the body of a response sent instead
// of an event stream is read up to a bound and still yields the status
func TestSSEClient_Query_TruncatesLongErrorBody(t *testing.T) {
	const prefix = "502 Bad Gateway: "
	_, err := queryAgainst(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		_, werr := fmt.Fprint(w, strings.Repeat("x", 3*maxErrorBodyBytes))
		assert.NoError(t, werr)
	})

	var statusErr huma.StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, http.StatusBadGateway, statusErr.GetStatus())
	assert.True(t, strings.HasPrefix(statusErr.Error(), prefix))
	assert.Len(t, statusErr.Error(), len(prefix)+maxErrorBodyBytes)
}

// TestSSEClient_Query_TransportStatusWins pins that the HTTP status of a rejection is
// authoritative over the status claimed inside its problem body
func TestSSEClient_Query_TransportStatusWins(t *testing.T) {
	_, err := queryAgainst(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnauthorized)
		_, werr := fmt.Fprint(w, `{"status":403,"title":"Forbidden","detail":"unauthenticated"}`)
		assert.NoError(t, werr)
	})

	var problem *query.DetailError
	require.ErrorAs(t, err, &problem)
	assert.Equal(t, http.StatusUnauthorized, problem.GetStatus())
	assert.Equal(t, http.StatusText(http.StatusUnauthorized), problem.Title)
	assert.Equal(t, "unauthenticated", problem.Detail)
}
