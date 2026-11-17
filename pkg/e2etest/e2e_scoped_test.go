//go:build !slimcap_nomock
// +build !slimcap_nomock

package e2etest

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/api/server"
	"github.com/els0r/goProbe/v4/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/goProbe/v4/plugins/authorizer/header"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every sensor of the scoped scenario captures its own pcap. Each fixture holds flows none of
// the others has, so a result that misses a sensor in the scope, or that carries the flows of the
// sensor outside it, shows in the rows
const (
	secondInScopePcapTestFile = "fragmented.pcap.gz"
	excludedPcapTestFile      = "c04-wap-r1.pcap.gz"
)

// TestE2EDistributedScoped pins the wiring of the header authorizer into a real global-query:
// a query for all hosts, sent with a scope naming two of three sensors, is answered with the
// data of those two sensors only. The sensors run in this process and therefore share its
// hostname, so the result rows and the successful per-host statuses carry a single hostname;
// the excluded sensor shows neither in the rows, nor in the summary totals, nor as a status
// keyed by its host ID
func TestE2EDistributedScoped(t *testing.T) {
	// Reset all Prometheus counters for the next E2E test to avoid double counting
	defer promMetrics.ResetCountersTestingOnly()

	// Sensors 0 and 1 are in the scope, sensor 2 is not
	var (
		scope    = []string{"0", "1"}
		excluded = "2"
		datasets = map[string][]byte{
			"0":      readPcap(t, defaultPcapTestFile),
			"1":      readPcap(t, secondInScopePcapTestFile),
			excluded: readPcap(t, excludedPcapTestFile),
		}

		endpoints   = make(map[string]string, len(datasets))
		sensorFlows = make(map[string]map[results.Attributes]struct{}, len(datasets))
		wantPackets uint64
	)
	for hostID, data := range datasets {
		endpoints[hostID] = fmt.Sprintf("127.0.0.1:%d", getAvailablePort())
		ifaces, _ := runSensor(t, hostID, endpoints[hostID], data)
		sensorFlows[hostID] = flows(ifaces)
		if hostID != excluded {
			wantPackets += ifaces.NParsed()
		}
	}

	authorizer, err := header.NewFromConfig(header.Config{Trusted: true})
	require.NoError(t, err)

	globalQueryEndpoint := fmt.Sprintf("127.0.0.1:%d", getAvailablePort())
	closeAPI := runGlobalQuery(t, globalQueryEndpoint, endpoints, server.WithAuthorizer(authorizer))
	defer func() {
		require.NoError(t, closeAPI())
	}()

	body, err := jsoniter.Marshal(query.Args{
		Query:      "sip,dip,dport,proto",
		Ifaces:     types.AnySelector,
		QueryHosts: types.AnySelector,
		Format:     types.FormatJSON,
		Last:       time.Now().Add(time.Hour).Format(time.ANSIC),
		NumResults: 100000,
		SortBy:     "packets",
		Caller:     "e2etest",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+globalQueryEndpoint+api.QueryRoute, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GoProbe-Allowed-Hosts", strings.Join(scope, ","))
	req.Header.Set("X-GoProbe-Principal", "e2etest")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	res := new(results.Result)
	require.NoError(t, jsoniter.NewDecoder(resp.Body).Decode(res))

	// Only the sensors in the scope contribute traffic
	assert.Equal(t, wantPackets, res.Summary.Totals.PacketsRcvd, "packets received must stem from the sensors in the scope only")

	// Every sensor has seen flows no other sensor has: a sensor in the scope that is not queried
	// (or that another one answers for) leaves its flows out of the rows, the excluded sensor
	// being queried puts its flows in
	onlySeenBy := func(hostID string, flow results.Attributes) bool {
		for other, seen := range sensorFlows {
			if _, ok := seen[flow]; ok && other != hostID {
				return false
			}
		}
		return true
	}
	for hostID, seen := range sensorFlows {
		distinctive := 0
		for flow := range seen {
			if onlySeenBy(hostID, flow) {
				distinctive++
			}
		}
		require.NotZero(t, distinctive, "the pcap of sensor %s must hold flows no other sensor has seen", hostID)
	}

	// The rows hold every flow of the sensors in the scope and none of the flows only the
	// excluded sensor has seen
	rowFlows := make(map[results.Attributes]struct{}, len(res.Rows))
	for _, row := range res.Rows {
		rowFlows[row.Attributes] = struct{}{}
	}
	for _, hostID := range scope {
		for flow := range sensorFlows[hostID] {
			assert.Contains(t, rowFlows, flow, "flow of sensor %s in the scope missing from the rows", hostID)
		}
	}
	for flow := range sensorFlows[excluded] {
		if onlySeenBy(excluded, flow) {
			assert.NotContains(t, rowFlows, flow, "flow only sensor %s outside the scope has seen is in the rows", excluded)
		}
	}

	// Rows carry the sensors' self-reported hostname, never a host ID
	hostname, err := os.Hostname()
	require.NoError(t, err)
	for _, row := range res.Rows {
		assert.Equal(t, hostname, row.Labels.Hostname)
	}

	// Successful statuses are keyed by hostname, failed ones by host ID: the sensors in the
	// scope share this process' hostname, the excluded sensor must not show up at all
	assert.Equal(t, results.HostsStatuses{hostname: {Code: types.StatusOK}}, res.HostsStatuses,
		"statuses must hold the in-scope sensors' hostname only, nothing keyed by host ID %s", excluded)
}

// readPcap returns the contents of a (compressed) pcap test fixture
func readPcap(t *testing.T, name string) []byte {
	t.Helper()
	data, err := pcaps.ReadFile(filepath.Join(testDataPath, name))
	require.NoError(t, err)
	return data
}

// flows returns the sip/dip/dport/proto tuples the mock interfaces have seen, in the shape the
// result rows report them
func flows(ifaces mockIfaces) map[results.Attributes]struct{} {
	out := make(map[results.Attributes]struct{})
	for _, iface := range ifaces {
		iface.RLock()
		for k := range *iface.flowsV4 {
			out[results.Attributes{
				SrcIP:   types.RawIPToAddr(k[capturetypes.EPHashV4SipStart:capturetypes.EPHashV4SipEnd]),
				DstIP:   types.RawIPToAddr(k[capturetypes.EPHashV4DipStart:capturetypes.EPHashV4DipEnd]),
				IPProto: k[capturetypes.EPHashV4ProtocolPos],
				DstPort: types.PortToUint16(k[capturetypes.EPHashV4DPortStart:capturetypes.EPHashV4DPortEnd]),
			}] = struct{}{}
		}
		for k := range *iface.flowsV6 {
			out[results.Attributes{
				SrcIP:   types.RawIPToAddr(k[capturetypes.EPHashV6SipStart:capturetypes.EPHashV6SipEnd]),
				DstIP:   types.RawIPToAddr(k[capturetypes.EPHashV6DipStart:capturetypes.EPHashV6DipEnd]),
				IPProto: k[capturetypes.EPHashV6ProtocolPos],
				DstPort: types.PortToUint16(k[capturetypes.EPHashV6DPortStart:capturetypes.EPHashV6DPortEnd]),
			}] = struct{}{}
		}
		iface.RUnlock()
	}
	return out
}
