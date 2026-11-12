//go:build !slimcap_nomock
// +build !slimcap_nomock

package e2etest

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/api/server"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/goProbe/v4/plugins/authorizer/header"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2EDistributedScoped pins the wiring of the header authorizer into a real global-query:
// a query for all hosts, sent with a scope naming two of three sensors, is answered with the
// data of those two sensors only. The sensors run in this process and therefore share its
// hostname, so the result rows and the successful per-host statuses carry a single hostname;
// the excluded sensor shows neither in the summary totals nor as a status keyed by its host ID
func TestE2EDistributedScoped(t *testing.T) {
	pcapData, err := pcaps.ReadFile(filepath.Join(testDataPath, defaultPcapTestFile))
	require.NoError(t, err)

	// Reset all Prometheus counters for the next E2E test to avoid double counting
	defer promMetrics.ResetCountersTestingOnly()

	const nSensors = 3
	sensors, endpoints := runSensors(t, nSensors, pcapData)
	scope := []string{"0", "1"}
	excluded := "2"

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

	req, err := http.NewRequest(http.MethodPost, "http://"+globalQueryEndpoint+api.QueryRoute, bytes.NewReader(body))
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

	// Only the sensors in the scope contribute traffic: each sensor captured the same pcap
	wantPackets := uint64(len(scope)) * sensors.NParsed()
	assert.Equal(t, wantPackets, res.Summary.Totals.PacketsRcvd, "packets received must stem from the %d sensors in the scope only", len(scope))
	require.NotEmpty(t, res.Rows)

	// Rows carry the sensors' self-reported hostname, never a host ID
	hostname, err := os.Hostname()
	require.NoError(t, err)
	for _, row := range res.Rows {
		assert.Equal(t, hostname, row.Labels.Hostname)
	}

	// Successful statuses are keyed by hostname, failed ones by host ID: the excluded sensor
	// must appear as neither
	assert.Equal(t, results.HostsStatuses{hostname: {Code: types.StatusOK}}, res.HostsStatuses)
	assert.NotContains(t, res.HostsStatuses, excluded)
}

// runSensors starts n goProbe sensors on the same pcap, each with its own DB and API, and
// returns the mock interfaces of the first one (all sensors see identical data) together with
// the API endpoints keyed by host ID
func runSensors(t *testing.T, n int, pcapData []byte) (mockIfaces, map[string]string) {
	t.Helper()

	var first mockIfaces
	endpoints := make(map[string]string, n)
	for i := range n {
		hostID := strconv.Itoa(i)
		ifaces := mockIfaces{newPcapSource(t, "mock001", pcapData)}
		if i == 0 {
			first = ifaces
		}

		endpoints[hostID] = fmt.Sprintf("127.0.0.1:%d", getAvailablePort())
		for range runGoProbe(t, t.TempDir(), endpoints[hostID], setupSources(ifaces)) {
			// drain the live flows: the DB on disk is what the query reads
		}
	}
	return first, endpoints
}
