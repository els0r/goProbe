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
	"github.com/els0r/goProbe/pkg/goDB/info"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/stretchr/testify/require"

	slimcap "github.com/fako1024/slimcap/capture"
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
	sourceInitFn func(c *capture.Capture) (slimcap.SourceZeroCopy, error)

	sync.RWMutex
}

type mockIfaces []*mockIface

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

func (m mockIfaces) BuildResults(t *testing.T, testDir string, resGoQuery results.Result) results.Result {

	res := results.Result{
		Status: results.Status{
			Code: types.StatusOK,
		},
		Summary: results.Summary{
			Interfaces: m.Names(),
		},
	}
	for _, iface := range m {
		iface.RLock()
		for k, v := range *iface.flows {
			res.Rows = append(res.Rows, results.Row{
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
			})
			res.Summary.Totals = res.Summary.Totals.Add(v)
		}
		iface.RUnlock()
	}

	// sort the results
	results.By(results.SortPackets, types.DirectionBoth, false).Sort(res.Rows)
	res.Summary.Hits.Total = len(res.Rows)
	res.Summary.Hits.Displayed = len(res.Rows)

	res.Query.Attributes = []string{"sip", "dip", "dport", "proto"}
	hostname, err := os.Hostname()
	require.Nil(t, err)
	for i := 0; i < len(res.Rows); i++ {
		res.Rows[i].Labels.HostID = info.GetHostID(testDir)
		res.Rows[i].Labels.Hostname = hostname
	}

	// Copy summary values that cannot be reproduced by the synthetic test
	res.Summary.First = resGoQuery.Summary.First
	res.Summary.Last = resGoQuery.Summary.Last
	res.Summary.Timings = resGoQuery.Summary.Timings

	return res
}

func (m mockIfaces) KillGoProbeOnceDone(cm *capture.Manager) {

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

		// Send the termination signal to goProbe
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGUSR2); err != nil {
			panic(err)
		}

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
