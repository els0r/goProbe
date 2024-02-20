//go:build !slimcap_nomock
// +build !slimcap_nomock

package e2etest

import (
	"context"
	"io/fs"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/conditions/node"
	"github.com/els0r/goProbe/pkg/goDB/info"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/stretchr/testify/require"
)

type mockTracking struct {
	nRead      uint64
	nProcessed uint64
	nErr       uint64

	done chan struct{}
}

type mockIface struct {
	name         string
	src          *afring.MockSource
	tracking     *mockTracking
	flows        *map[capturetypes.EPHash]types.Counters
	sourceInitFn func(c *capture.Capture) (capture.Source, error)

	sync.RWMutex
}

type mockIfaces []*mockIface

func (m *mockIface) aggregate() hashmap.AggFlowMapWithMetadata {

	result := hashmap.NewAggFlowMap()

	// Reusable key conversion buffers
	keyBufV4, keyBufV6 := types.NewEmptyV4Key(), types.NewEmptyV6Key()
	for k, v := range *m.flows {

		if types.RawIPToAddr(k[0:16]).Is4() && types.RawIPToAddr(k[16:32]).Is4() {
			keyBufV4.PutAllV4(k[0:4], k[16:20], k[32:34], k[36])
			result.SetOrUpdate(keyBufV4, true, v.BytesRcvd, v.BytesSent, v.PacketsRcvd, v.PacketsSent)
		} else {
			keyBufV6.PutAllV6(k[0:16], k[16:32], k[32:34], k[36])
			result.SetOrUpdate(keyBufV6, false, v.BytesRcvd, v.BytesSent, v.PacketsRcvd, v.PacketsSent)
		}
	}

	return hashmap.AggFlowMapWithMetadata{
		AggFlowMap: result,
		Interface:  m.name,
	}
}

func (m mockIfaces) Names() (names []string) {
	names = make([]string, len(m))
	for i := 0; i < len(m); i++ {
		names[i] = m[i].name
	}
	return
}

func (m mockIfaces) WaitUntilDoneReading() {

	wg := sync.WaitGroup{}
	wg.Add(len(m))

	for i := 0; i < len(m); i++ {
		go func(i int) {
			<-m[i].tracking.done
			wg.Done()
		}(i)
	}

	wg.Wait()
}

func (m mockIfaces) NRead() (res uint64) {
	for _, v := range m {
		v.RLock()
		res += v.tracking.nRead
		v.RUnlock()
	}
	return
}

func (m mockIfaces) NProcessed() (res uint64) {
	for _, v := range m {
		v.RLock()
		res += v.tracking.nProcessed
		v.RUnlock()
	}
	return
}

func (m mockIfaces) NErr() (res uint64) {
	for _, v := range m {
		v.RLock()
		res += v.tracking.nErr
		v.RUnlock()
	}
	return
}

func (m mockIfaces) BuildResults(t *testing.T, testDir string, valFilterNode *node.ValFilterNode, resGoQuery *results.Result) (results.Result, []goDB.InterfaceMetadata) {

	res := results.Result{
		Status: results.Status{
			Code: types.StatusOK,
		},
		Summary: results.Summary{
			Interfaces:    m.Names(),
			DataAvailable: true,
		},
	}
	ifaceMetadata := make([]goDB.InterfaceMetadata, len(m))
	for i, iface := range m {
		iface.RLock()
		ifaceMetadata[i].Iface = iface.name

		// Copy summary values that cannot be reproduced by the synthetic test
		ifaceMetadata[i].First = resGoQuery.Summary.First
		ifaceMetadata[i].Last = resGoQuery.Summary.Last

		for k, v := range *iface.flows {
			row := results.Row{
				Labels: results.Labels{
					Iface: iface.name,
				},
				Attributes: results.Attributes{
					SrcIP:   types.RawIPToAddr(k[0:16]),
					DstIP:   types.RawIPToAddr(k[16:32]),
					IPProto: k[36],
					DstPort: types.PortToUint16(k[32:34]),
				},
				Counters: v,
			}
			if valFilterNode == nil || valFilterNode.ValFilter(row.Counters) {
				res.Rows = append(res.Rows, row)
				res.Summary.Totals = res.Summary.Totals.Add(v)
			}
			ifaceMetadata[i].Counts = ifaceMetadata[i].Counts.Add(v)
			if row.Attributes.SrcIP.Is4() && row.Attributes.DstIP.Is4() {
				ifaceMetadata[i].Traffic.NumV4Entries++
			} else {
				ifaceMetadata[i].Traffic.NumV6Entries++
			}
		}
		iface.RUnlock()
	}

	// sort the results
	results.By(results.SortPackets, types.DirectionBoth, false).Sort(res.Rows)
	res.Summary.Hits.Total = len(res.Rows)
	res.Summary.Hits.Displayed = len(res.Rows)

	res.Query.Attributes = []string{types.SIPName, types.DIPName, types.DportName, types.ProtoName}
	hostname, err := os.Hostname()
	require.Nil(t, err)
	hostID := info.GetHostID(testDir)
	for i := 0; i < len(res.Rows); i++ {
		res.Rows[i].Labels.HostID = hostID
		res.Rows[i].Labels.Hostname = hostname
	}

	// Copy summary values that cannot be reproduced by the synthetic test
	res.Summary.First = resGoQuery.Summary.First
	res.Summary.Last = resGoQuery.Summary.Last
	res.Summary.Timings = resGoQuery.Summary.Timings

	return res, ifaceMetadata
}

func (m mockIfaces) KillGoProbeOnceDone(cm *capture.Manager, flows chan hashmap.AggFlowMapWithMetadata) {

	// Wait until all mock data has been consumed (e.g. from a pcap file)
	m.WaitUntilDoneReading()
	nRead := m.NRead()

	ctx := context.Background()
	for {
		time.Sleep(50 * time.Millisecond)

		// Grab the number of overall received / processed packets in all captures and
		// wait until they match the number of read packets
		var nProcessed uint64
		for _, st := range cm.Status(ctx) {
			nProcessed += st.ProcessedTotal
		}
		if nRead == 0 || nProcessed != nRead {
			continue
		}

		cm.GetFlowMaps(ctx, nil, flows)

		// Send the termination signal to goProbe
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGUSR2); err != nil {
			panic(err)
		}

		close(flows)

		return
	}
}

// Perm calls f with each permutation of a.
func Perm(a []fs.DirEntry, f func([]fs.DirEntry)) {
	for i := 1; i <= len(a); i++ {
		perm(a[:i], f, 0)
	}
}

// Permute the values at index i to len(a)-1.
func perm(a []fs.DirEntry, f func([]fs.DirEntry), i int) {
	if i > len(a) {
		f(a)
		return
	}
	perm(a, f, i+1)
	for j := i + 1; j < len(a); j++ {
		a[i], a[j] = a[j], a[i]
		perm(a, f, i+1)
		a[i], a[j] = a[j], a[i]
	}
}
