//go:build !slimcap_nomock
// +build !slimcap_nomock

package capture

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goprobe/writeout"
	"github.com/els0r/telemetry/logging"
	"github.com/fako1024/slimcap/capture"
	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/fako1024/slimcap/link"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/ipv4"
)

const randSeed = 10000

var defaultMockIfaceConfig = config.CaptureConfig{
	Promisc: false,
	RingBuffer: &config.RingBufferConfig{
		BlockSize: config.DefaultRingBufferBlockSize,
		NumBlocks: config.DefaultRingBufferNumBlocks,
	},
}

type testMockSrc struct {
	src     *afring.MockSourceNoDrain
	errChan <-chan error
}

type testMockSrcs map[string]testMockSrc

func (t testMockSrcs) Done() {
	for _, src := range t {
		src.src.Done()
	}
}

func (t testMockSrcs) Wait() error {
	for _, src := range t {
		if err := <-src.errChan; err != nil {
			return err
		}
	}
	return nil
}

func TestConcurrentMethodAccess(t *testing.T) {

	require.Nil(t, logging.Init(logging.LevelWarn, logging.EncodingLogfmt,
		logging.WithOutput(os.Stdout),
		logging.WithErrorOutput(os.Stderr),
	))

	for _, i := range []int{1, 2, 3, 10} {
		t.Run(fmt.Sprintf("%d ifaces", i), func(t *testing.T) {
			testConcurrentMethodAccess(t, i, 1000)
		})
	}
}

func testConcurrentMethodAccess(t *testing.T, nIfaces, nIterations int) {

	captureManager, ifaceConfigs, testMockSrcs := setupInterfaces(t, defaultMockIfaceConfig, nIfaces)

	time.Sleep(time.Second)

	wg := sync.WaitGroup{}
	wg.Add(3)

	var errCount uint64
	go func() {
		ctx := context.Background()
		prng := rand.New(rand.NewSource(randSeed)) // #nosec G404
		for i := 0; i < nIterations; i++ {
			ifaceIdx := prng.Int63n(int64(nIfaces))
			captureManager.Status(ctx, fmt.Sprintf("mock%00d", ifaceIdx))
		}
		wg.Done()
	}()

	go func() {
		ctx := context.Background()
		writeoutChan := make(chan capturetypes.TaggedAggFlowMap, 1)
		prng := rand.New(rand.NewSource(randSeed)) // #nosec G404
		for i := 0; i < nIterations; i++ {
			ifaceIdx := prng.Int63n(int64(nIfaces))
			captureManager.rotate(ctx, writeoutChan, fmt.Sprintf("mock%00d", ifaceIdx))
			<-writeoutChan
		}
		wg.Done()
	}()

	go func() {
		ctx := context.Background()
		for i := 0; i < nIterations; i++ {
			if _, _, _, err := captureManager.Update(ctx, ifaceConfigs); err != nil {
				atomic.AddUint64(&errCount, 1)
			}
		}
		wg.Done()
	}()

	wg.Wait()

	require.Zero(t, atomic.LoadUint64(&errCount))
	testMockSrcs.Done()
	require.Nil(t, testMockSrcs.Wait())

	captureManager.Close(context.Background())
}

func setupInterfaces(t *testing.T, cfg config.CaptureConfig, nIfaces int) (*Manager, config.Ifaces, testMockSrcs) {

	ifaceConfigs := make(config.Ifaces)
	for i := 0; i < nIfaces; i++ {
		ifaceConfigs[fmt.Sprintf("mock%00d", i)] = cfg
	}

	// Setup a temporary directory for the test DB
	tempDir, err := os.MkdirTemp(os.TempDir(), "goprobe_capture")
	require.Nil(t, err)
	defer func(t *testing.T) {
		require.Nil(t, os.RemoveAll(tempDir))
	}(t)

	// Build / initialize mock sources for all interfaces
	testMockSrcs := make(testMockSrcs)
	for iface := range ifaceConfigs {
		mockSrc, errChan := initMockSrc(t, iface)
		testMockSrcs[iface] = testMockSrc{
			src:     mockSrc,
			errChan: errChan,
		}
	}

	// Initialize the CaptureManager
	captureManager := NewManager(
		writeout.NewGoDBHandler(tempDir, encoders.EncoderTypeLZ4),
		WithSourceInitFn(func(c *Capture) (capture.SourceZeroCopy, error) {
			src, exists := testMockSrcs[c.Iface()]
			if !exists {
				return nil, fmt.Errorf("failed to initialize missing interface %s", c.Iface())
			}

			return src.src, nil
		}),
	)
	_, _, _, err = captureManager.Update(context.Background(), ifaceConfigs)
	require.Nil(t, err)

	return captureManager, ifaceConfigs, testMockSrcs
}

func TestLowTrafficDeadlock(t *testing.T) {
	for _, n := range []int{0, 1, 10, 100, 1000} {
		t.Run(fmt.Sprintf("%d packets", n), func(t *testing.T) {
			testDeadlockLowTraffic(t, n)
		})
	}
}

func TestHighTrafficDeadlock(t *testing.T) {
	testDeadlockHighTraffic(t)
}

func TestMockPacketCapturePerformance(t *testing.T) {

	if testing.Short() {
		t.SkipNow()
	}

	testPacket, err := genDummyPacket()
	require.Nil(t, err)

	mockSrc, err := afring.NewMockSourceNoDrain("mock",
		afring.CaptureLength(link.CaptureLengthMinimalIPv4Transport),
	)
	require.Nil(t, err)
	mockC := newMockCapture(mockSrc)

	for mockSrc.CanAddPackets() {
		require.Nil(t, mockSrc.AddPacket(testPacket))
	}
	errChan, err := mockSrc.Run(time.Microsecond)
	require.Nil(t, err)

	runtime := 10 * time.Second
	mockC.process()
	time.Sleep(runtime)

	mockSrc.Done()
	<-errChan

	mockC.lock()
	flowsV4, flowsV6 := mockC.flowLog.FlowsV4(), mockC.flowLog.FlowsV6()
	for _, v := range flowsV4 {
		fmt.Printf("IPv4 Packets processed after %v: %d (%v/pkt)\n", runtime, v.PacketsSent, runtime/time.Duration(v.PacketsSent))
	}
	for _, v := range flowsV6 {
		fmt.Printf("IPv6 Packets processed after %v: %d (%v/pkt)\n", runtime, v.PacketsSent, runtime/time.Duration(v.PacketsSent))
	}
	mockC.unlock()

	require.Nil(t, mockC.close())
}

func BenchmarkRotation(b *testing.B) {

	b.Run("Minimal", func(b *testing.B) {
		benchmarkRotation(b, 10, 10, 10)
	})

	b.Run("ManyTalkersFewApplications", func(b *testing.B) {
		benchmarkRotation(b, 2500, 10, 10)
	})

	b.Run("FewTalkersManyPortPairs", func(b *testing.B) {
		benchmarkRotation(b, 10, 250, 250)
	})

	b.Run("FewTalkersManySourcePorts", func(b *testing.B) {
		benchmarkRotation(b, 10, 2500, 10)
	})

	b.Run("FewTalkersManyDestinationPorts", func(b *testing.B) {
		benchmarkRotation(b, 10, 10, 2500)
	})

	b.Run("ManyTalkersManySourcePorts", func(b *testing.B) {
		benchmarkRotation(b, 250, 250, 10)
	})

	b.Run("ManyTalkersManyDestinationPorts", func(b *testing.B) {
		benchmarkRotation(b, 400, 10, 400)
	})
}

func benchmarkRotation(b *testing.B, nIP uint32, nSPort, nDPort uint16) {

	pkt, err := capture.BuildPacket(
		net.ParseIP("1.2.3.4"),
		net.ParseIP("4.5.6.7"),
		1,
		2,
		17, []byte{1, 2}, capture.PacketOutgoing, 128)

	require.Nil(b, err)
	ipLayer := pkt.IPLayer()

	benchCap := &Capture{
		flowLog: NewFlowLog(),
	}

	for ip := uint32(1); ip <= nIP; ip++ {
		for s := uint16(1); s <= nSPort; s++ {
			for d := uint16(1); d <= nDPort; d++ {
				binary.BigEndian.PutUint32(ipLayer[16:20], ip)
				binary.BigEndian.PutUint16(ipLayer[ipv4.HeaderLen:ipv4.HeaderLen+2], s)
				binary.BigEndian.PutUint16(ipLayer[ipv4.HeaderLen+2:ipv4.HeaderLen+4], d)

				epHash, auxInfo, errno := ParsePacketV4(ipLayer)

				switch {

				// If the destination port is a common one, we expect a non-reverse situation
				case isCommonPort(epHash[10:12], 17):
					if epHash.IsProbablyReverse() {
						b.Fatalf("unexpectedly detected probably reverse packet for %s", ipLayer.String())
					}

				// If the source port is a common one, we expect a reverse situation
				case isCommonPort(epHash[4:6], 17):
					if !epHash.IsProbablyReverse() {
						b.Fatalf("unexpectedly didn't detect probably reverse packet for %s", ipLayer.String())
					}

				// If the source port is smaller than the destination portm, we expect a reverse situation
				case s < d:
					if !epHash.IsProbablyReverse() {
						b.Fatalf("unexpectedly didn't detect probably reverse packet for %s", ipLayer.String())
					}

				// Anything else should be a non-reverse situation
				default:
					if epHash.IsProbablyReverse() {
						b.Fatalf("unexpectedly detected probably reverse packet for %s", ipLayer.String())
					}
				}

				benchCap.addToFlowLogV4(epHash, capture.PacketOutgoing, 128, auxInfo, errno)
			}
		}
	}

	printMemUsage(benchCap.flowLog)

	b.Run("pre_add_req", func(b *testing.B) {
		pkt, err = capture.BuildPacket(
			net.ParseIP("100.2.3.4"),
			net.ParseIP("100.5.6.7"),
			10000,
			444,
			17, []byte{1, 2}, capture.PacketOutgoing, 128)
		require.Nil(b, err)

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			epHash, auxInfo, errno := ParsePacketV4(pkt.IPLayer())
			benchCap.addToFlowLogV4(epHash, capture.PacketOutgoing, 128, auxInfo, errno)
		}
	})

	b.Run("pre_add_resp", func(b *testing.B) {
		pkt, err = capture.BuildPacket(
			net.ParseIP("100.5.6.7"),
			net.ParseIP("100.2.3.4"),
			444,
			10000,
			17, []byte{1, 2}, capture.PacketThisHost, 128)
		require.Nil(b, err)

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			epHash, auxInfo, errno := ParsePacketV4(pkt.IPLayer())
			benchCap.addToFlowLogV4(epHash, capture.PacketThisHost, 128, auxInfo, errno)
		}
	})

	b.Run("rotation", func(b *testing.B) {

		benchData := make([]*FlowLog, b.N)
		for i := 0; i < len(benchData); i++ {
			benchData[i] = benchCap.flowLog.clone()
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {

			// Run best-case scenario (keep all flows)
			aggMap, _ := benchData[i].transferAndAggregate()
			_ = aggMap

			// Run worst-case scenario (keep no flows)
			aggMap, _ = benchData[i].transferAndAggregate()
			_ = aggMap
		}
	})

	printMemUsage(benchCap.flowLog)

	testLog := benchCap.flowLog.clone()
	testLog.transferAndAggregate()
	testLog.transferAndAggregate()

	benchCapPost := &Capture{
		flowLog: testLog,
	}

	b.Run("post_add_req", func(b *testing.B) {
		pkt, err = capture.BuildPacket(
			net.ParseIP("200.2.3.4"),
			net.ParseIP("200.5.6.7"),
			10000,
			444,
			17, []byte{1, 2}, capture.PacketOutgoing, 128)
		require.Nil(b, err)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			epHash, auxInfo, errno := ParsePacketV4(pkt.IPLayer())
			benchCapPost.addToFlowLogV4(epHash, capture.PacketOutgoing, 128, auxInfo, errno)
		}
	})

	b.Run("post_add_resp", func(b *testing.B) {
		pkt, err = capture.BuildPacket(
			net.ParseIP("200.5.6.7"),
			net.ParseIP("200.2.3.4"),
			444,
			10000,
			17, []byte{1, 2}, capture.PacketThisHost, 128)
		require.Nil(b, err)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			epHash, auxInfo, errno := ParsePacketV4(pkt.IPLayer())
			benchCapPost.addToFlowLogV4(epHash, capture.PacketThisHost, 128, auxInfo, errno)
		}
	})
}

func testDeadlockLowTraffic(t *testing.T, maxPkts int) {

	ctx := context.Background()
	testPacket, err := genDummyPacket()
	require.Nil(t, err)

	mockSrc, err := afring.NewMockSource("mock",
		afring.CaptureLength(link.CaptureLengthMinimalIPv4Transport),
	)
	require.Nil(t, err)

	errChan := mockSrc.Run()
	go func() {
		for i := 0; i < maxPkts; i++ {
			require.Nil(t, mockSrc.AddPacket(testPacket))
		}
		mockSrc.Done()
	}()

	mockC := newMockCapture(mockSrc)
	mockC.process()

	start := time.Now()
	doneChan := make(chan error)
	time.AfterFunc(100*time.Millisecond, func() {

		t.Logf("starting roation loops after %v", time.Since(start))
		for i := 0; i < 20; i++ {
			mockC.lock()
			mockC.rotate(ctx)
			mockC.unlock()
			time.Sleep(10 * time.Millisecond)
		}
		t.Logf("roation loops done after %v", time.Since(start))

		select {
		case err := <-errChan:
			doneChan <- err
		case <-time.After(10 * time.Second):
			doneChan <- errors.New("potential deadlock situation on rotation logic (no termination confirmation received from mock source)")
		}

		require.Nil(t, mockC.close())
	})

	require.Nil(t, <-doneChan)

	if time.Since(start) > 10*time.Second {
		t.Fatalf("potential deadlock situation on rotation logic (test took %v)", time.Since(start))
	}
}

func testDeadlockHighTraffic(t *testing.T) {

	ctx := context.Background()
	testPacket, err := genDummyPacket()
	require.Nil(t, err)

	mockSrc, err := afring.NewMockSourceNoDrain("mock",
		afring.CaptureLength(link.CaptureLengthMinimalIPv4Transport),
	)
	require.Nil(t, err)
	mockC := newMockCapture(mockSrc)

	for mockSrc.CanAddPackets() {
		require.Nil(t, mockSrc.AddPacket(testPacket))
	}
	errChan, err := mockSrc.Run(time.Microsecond)
	require.Nil(t, err)

	mockC.process()

	start := time.Now()
	doneChan := make(chan error)
	time.AfterFunc(100*time.Millisecond, func() {

		t.Logf("starting roation loops after %v", time.Since(start))
		for i := 0; i < 20; i++ {
			mockC.lock()
			mockC.rotate(ctx)
			mockC.unlock()
			time.Sleep(10 * time.Millisecond)
		}
		mockSrc.Done()
		t.Logf("roation loops done after %v", time.Since(start))

		select {
		case err := <-errChan:
			doneChan <- err
		case <-time.After(time.Minute):
			doneChan <- errors.New("potential deadlock situation on rotation logic (no termination confirmation received from mock source)")
		}

		require.Nil(t, mockC.close())
	})

	require.Nil(t, <-doneChan)

	if time.Since(start) > 3*time.Minute {
		t.Fatalf("potential deadlock situation on rotation logic (test took %v)", time.Since(start))
	}
}

func newMockCapture(src capture.SourceZeroCopy) *Capture {
	return &Capture{
		iface:         src.Link().Name,
		capLock:       newCaptureLock(),
		flowLog:       NewFlowLog(),
		captureHandle: src,
	}
}

func genDummyPacket() (capture.Packet, error) {
	return capture.BuildPacket(
		net.ParseIP("1.2.3.4"),
		net.ParseIP("4.5.6.7"),
		1,
		2,
		6, []byte{1, 2}, capture.PacketOutgoing, 128)
}

func initMockSrc(t *testing.T, iface string) (*afring.MockSourceNoDrain, <-chan error) {

	testPacket, err := genDummyPacket()
	require.Nil(t, err)

	mockSrc, err := afring.NewMockSourceNoDrain(iface,
		afring.CaptureLength(link.CaptureLengthMinimalIPv4Transport),
	)
	require.Nil(t, err)
	for mockSrc.CanAddPackets() {
		require.Nil(t, mockSrc.AddPacket(testPacket))
	}

	errChan, err := mockSrc.Run(100 * time.Millisecond)
	require.Nil(t, err)

	return mockSrc, errChan
}

func printMemUsage(flowLog *FlowLog) {

	runtime.GC()
	debug.FreeOSMemory()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("# Flow Logs = %d", flowLog.Len())
	fmt.Printf("\tAlloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
