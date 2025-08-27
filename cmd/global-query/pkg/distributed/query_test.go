package distributed

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/goProbe/v4/plugins/resolver/stringresolver"
	"github.com/stretchr/testify/require"
)

// mockResolver is a simple Resolver that records calls and returns a preset list
type mockResolver struct {
	calledWith string
	calls      int
	out        hosts.Hosts
	err        error
}

func (m *mockResolver) Resolve(_ context.Context, query string) (hosts.Hosts, error) {
	m.calls++
	m.calledWith = query
	return m.out, m.err
}

// mockQuerier implements distributed.Querier and optionally distributed.QuerierAnyable
type mockQuerier struct {
	lastCtx   context.Context
	lastHosts hosts.Hosts
	lastArgs  *query.Args
	results   []*results.Result
	anyHosts  hosts.Hosts
}

func (m *mockQuerier) Query(ctx context.Context, queryHosts hosts.Hosts, args *query.Args) (<-chan *results.Result, <-chan struct{}) {
	m.lastCtx = ctx
	m.lastHosts = queryHosts
	m.lastArgs = args
	rc := make(chan *results.Result, len(m.results))
	kc := make(chan struct{})
	for _, r := range m.results {
		rc <- r
	}
	close(rc)
	close(kc)
	return rc, kc
}

func (m *mockQuerier) AllHosts() (hosts.Hosts, error) { // implements distributed.QuerierAnyable
	return m.anyHosts, nil
}

func makeResult(host, iface string, bytes, packets uint64) *results.Result {
	r := results.New()
	r.Start()
	r.Hostname = host
	r.Rows = results.Rows{
		{
			Labels:     results.Labels{Iface: iface, Hostname: host},
			Attributes: results.Attributes{DstPort: 80},
			Counters:   types.Counters{BytesRcvd: bytes, PacketsRcvd: packets},
		},
	}
	r.Summary.Interfaces = []string{iface}
	r.Summary.First = time.Now().Add(-time.Hour)
	r.Summary.Last = time.Now()
	r.Summary.Hits.Total = len(r.Rows)
	return r
}

func baseArgs() *query.Args {
	return &query.Args{
		Query:      "sip",
		Ifaces:     "eth0",
		Format:     "json",
		MaxMemPct:  60,
		NumResults: 100,
		First:      time.Now().Add(-time.Hour).Format(time.RFC3339),
		Last:       time.Now().Format(time.RFC3339),
	}
}

func TestRun_ErrorWhenQueryHostsEmpty(t *testing.T) {
	// Resolver map can be empty; early error occurs before resolver lookup
	rm := hosts.NewResolverMap()
	mq := &mockQuerier{}
	qr := NewQueryRunner(rm, mq)

	args := baseArgs()
	args.QueryHosts = ""

	res, err := qr.Run(context.Background(), args)
	require.Nil(t, res)
	require.Error(t, err)
	require.Contains(t, err.Error(), "query for target hosts is empty")
	// resolver not looked up/called
}

func TestRun_UsesResolverAndAggregatesResults(t *testing.T) {
	mr := &mockResolver{out: hosts.Hosts{"h1", "h2"}}
	rm := hosts.NewResolverMap()
	rm.Set("string", mr)
	mq := &mockQuerier{results: []*results.Result{
		makeResult("h1", "eth0", 10, 1),
		makeResult("h2", "eth0", 20, 2),
	}}
	qr := NewQueryRunner(rm, mq)

	args := baseArgs()
	args.QueryHosts = "h1,h2"

	res, err := qr.Run(context.Background(), args)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Resolver was used
	require.Equal(t, 1, mr.calls)
	require.Equal(t, "h1,h2", mr.calledWith)
	require.ElementsMatch(t, mr.out, mq.lastHosts)

	// Aggregation combined rows and interfaces, and totals reflect both inputs
	require.Equal(t, 2, res.Summary.Hits.Total)
	require.Len(t, res.Rows, 2)
	require.ElementsMatch(t, []string{"eth0"}, res.Summary.Interfaces)
}

func TestRun_AnySelector_UsesQuerierAnyable(t *testing.T) {
	// Include a "string" resolver but it should not be used for Any selector
	mr := &mockResolver{out: hosts.Hosts{"should-not-be-used"}}
	rm := hosts.NewResolverMap()
	rm.Set("string", mr)
	mq := &mockQuerier{anyHosts: hosts.Hosts{"any1", "any2"}, results: []*results.Result{
		makeResult("any1", "eth0", 5, 1),
		makeResult("any2", "eth0", 7, 1),
	}}
	qr := NewQueryRunner(rm, mq)

	args := baseArgs()
	args.QueryHosts = types.AnySelector

	res, err := qr.Run(context.Background(), args)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Resolver should not have been called for Any selector
	require.Equal(t, 0, mr.calls)
	require.ElementsMatch(t, mq.anyHosts, mq.lastHosts)
}

func TestRun_StringResolverOverride(t *testing.T) {
	// Use the real string resolver to validate sorting & deduplication
	rm := hosts.NewResolverMap()
	rm.Set(stringresolver.Type, stringresolver.NewResolver(true))
	mq := &mockQuerier{results: []*results.Result{makeResult("a", "eth0", 1, 1)}}
	qr := NewQueryRunner(rm, mq)

	args := baseArgs()
	args.QueryHosts = "b, a, a"
	args.QueryHostsResolverType = stringresolver.Type

	res, err := qr.Run(context.Background(), args)
	require.NoError(t, err)
	require.NotNil(t, res)

	// The used hosts should be sorted & de-duplicated by string resolver
	require.ElementsMatch(t, hosts.Hosts{"a", "b"}, mq.lastHosts)
}

// captureSender captures SSE messages via the Sender function
type captureSender struct {
	mu     sync.Mutex
	events []struct {
		kind string
		rows int
	}
}

func (c *captureSender) send(msg sse.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch v := msg.Data.(type) {
	case *api.PartialResult:
		c.events = append(c.events, struct {
			kind string
			rows int
		}{kind: "partial", rows: len(v.Rows)})
	case *api.Keepalive:
		c.events = append(c.events, struct {
			kind string
			rows int
		}{kind: "keepalive", rows: 0})
	}
	return nil
}

// scriptedQuerier lets tests control result and keepalive emissions
type scriptedQuerier struct {
	rc chan *results.Result
	kc chan struct{}
}

func (sq *scriptedQuerier) Query(_ context.Context, _ hosts.Hosts, _ *query.Args) (<-chan *results.Result, <-chan struct{}) {
	return sq.rc, sq.kc
}

func TestRunStreaming_PartialResults_IncludeRows(t *testing.T) {
	// Arrange
	mr := &mockResolver{out: hosts.Hosts{"h1", "h2"}}
	rm := hosts.NewResolverMap()
	rm.Set("string", mr)
	sq := &scriptedQuerier{rc: make(chan *results.Result, 2), kc: make(chan struct{}, 1)}
	qr := NewQueryRunner(rm, sq)

	args := baseArgs()
	args.QueryHosts = "h1,h2"

	sender := &captureSender{}

	// Act: send two per-host results, then close
	sq.rc <- makeResult("h1", "eth0", 10, 1)
	sq.rc <- makeResult("h2", "eth0", 20, 2)
	close(sq.rc)
	close(sq.kc)

	res, err := qr.RunStreaming(context.Background(), args, sse.Sender(sender.send))

	// Assert
	require.NoError(t, err)
	require.NotNil(t, res)

	// Expect two PartialResult SSE events with increasing row counts
	var rowCounts []int
	for _, ev := range sender.events {
		if ev.kind == "partial" {
			rowCounts = append(rowCounts, ev.rows)
		}
	}
	require.Len(t, rowCounts, 2)
	require.Equal(t, 1, rowCounts[0])
	require.Equal(t, 2, rowCounts[1])
}

func TestRunStreaming_Keepalives_AreEmitted(t *testing.T) {
	// Arrange
	mr := &mockResolver{out: hosts.Hosts{"h1"}}
	rm := hosts.NewResolverMap()
	rm.Set("string", mr)
	// unbuffered result channel to keep aggregation running while we emit keepalives
	rc := make(chan *results.Result)
	kc := make(chan struct{}, 8)
	sq := &scriptedQuerier{rc: rc, kc: kc}
	qr := NewQueryRunner(rm, sq)

	args := baseArgs()
	args.QueryHosts = "h1"
	args.KeepAlive = 5 * time.Millisecond

	sender := &captureSender{}

	// Emit keepalives with spacing > keepalive interval to trigger sends,
	// then send a result and close channels to finish.
	go func() {
		// allow forwardKeepalives goroutine to start
		time.Sleep(2 * time.Millisecond)
		kc <- struct{}{}
		time.Sleep(12 * time.Millisecond)
		kc <- struct{}{}
		time.Sleep(12 * time.Millisecond)
		kc <- struct{}{}
		close(kc)
		// now allow aggregation to complete
		rc <- makeResult("h1", "eth0", 1, 1)
		close(rc)
	}()

	res, err := qr.RunStreaming(context.Background(), args, sse.Sender(sender.send))
	require.NoError(t, err)
	require.NotNil(t, res)

	// Assert at least one keepalive event was emitted.
	// Note: exact counts are timing-sensitive and can be flaky across environments,
	// so we only verify presence rather than a specific minimum beyond 1.
	keepaliveCount := 0
	for _, ev := range sender.events {
		if ev.kind == "keepalive" {
			keepaliveCount++
		}
	}
	require.GreaterOrEqual(t, keepaliveCount, 1, sender.events)
}
