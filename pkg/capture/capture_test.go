package capture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goprobe/writeout"
	"github.com/fako1024/slimcap/capture"
	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/fako1024/slimcap/link"
	"github.com/stretchr/testify/require"
)

func TestConcurrentMethodAccess(t *testing.T) {

	ifaceConfig := config.Ifaces{
		"mock": config.CaptureConfig{
			Promisc:             false,
			RingBufferBlockSize: config.DefaultRingBufferSize,
			RingBufferNumBlocks: 4,
		},
	}

	// Setup a temporary directory for the test DB
	tempDir, err := os.MkdirTemp(os.TempDir(), "goprobe_capture")
	if err != nil {
		panic(err)
	}
	defer require.Nil(t, os.RemoveAll(tempDir))

	testPacket, err := genDummyPacket()
	require.Nil(t, err)

	mockSrc, err := afring.NewMockSourceNoDrain("mock",
		afring.CaptureLength(link.CaptureLengthMinimalIPv4Transport),
	)
	require.Nil(t, err)
	for mockSrc.CanAddPackets() {
		require.Nil(t, mockSrc.AddPacket(testPacket))
	}
	errChan := mockSrc.Run(time.Microsecond)

	// Initialize the CaptureManager
	captureManager := NewManager(
		writeout.NewGoDBHandler(tempDir, encoders.EncoderTypeLZ4),
		WithSourceInitFn(func(c *Capture) (capture.Source, error) {
			return mockSrc, nil
		}),
	)
	captureManager.Update(context.Background(), ifaceConfig)

	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		ctx := context.Background()
		for i := 0; i < 10000; i++ {
			captureManager.Status(ctx, "mock")
		}
		wg.Done()
	}()

	go func() {
		ctx := context.Background()
		writeoutChan := make(chan capturetypes.TaggedAggFlowMap, 1)
		for i := 0; i < 10000; i++ {
			captureManager.rotate(ctx, writeoutChan, "mock")
			<-writeoutChan
		}
		wg.Done()
	}()

	go func() {
		ctx := context.Background()
		for i := 0; i < 10000; i++ {
			captureManager.Update(ctx, ifaceConfig)
		}
		wg.Done()
	}()

	wg.Wait()

	mockSrc.Done()
	<-errChan

	captureManager.Close(context.Background())
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

	ctx := context.Background()

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
	errChan := mockSrc.Run(time.Microsecond)

	runtime := 10 * time.Second
	mockC.process(ctx)
	time.Sleep(runtime)

	mockSrc.Done()
	<-errChan

	mockC.lock()
	flows := mockC.flowLog.Flows()
	for _, v := range flows {
		fmt.Printf("Packets processed after %v: %d (%v/pkt)\n", runtime, v.PacketsSent(), runtime/time.Duration(v.PacketsSent()))
	}
	mockC.unlock()

	require.Nil(t, mockC.close())
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
	mockC.process(ctx)

	start := time.Now()
	doneChan := make(chan error)
	time.AfterFunc(100*time.Millisecond, func() {
		for i := 0; i < 20; i++ {
			mockC.lock()
			mockC.rotate(ctx)
			mockC.unlock()
			time.Sleep(10 * time.Millisecond)
		}

		select {
		case err := <-errChan:
			doneChan <- err
			break
		case <-time.After(10 * time.Second):
			doneChan <- errors.New("potential deadlock situation on rotation logic")
		}

		require.Nil(t, mockC.close())
	})

	require.Nil(t, <-doneChan)

	if time.Since(start) > 2*time.Second {
		t.Fatalf("potential deadlock situation on rotation logic")
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
	errChan := mockSrc.Run(time.Microsecond)

	mockC.process(ctx)

	start := time.Now()
	doneChan := make(chan error)
	time.AfterFunc(100*time.Millisecond, func() {
		for i := 0; i < 20; i++ {
			mockC.lock()
			mockC.rotate(ctx)
			mockC.unlock()
			time.Sleep(10 * time.Millisecond)
		}
		mockSrc.Done()

		select {
		case err := <-errChan:
			doneChan <- err
			break
		case <-time.After(10 * time.Second):
			doneChan <- errors.New("potential deadlock situation on rotation logic")
		}

		require.Nil(t, mockC.close())
	})

	require.Nil(t, <-doneChan)

	if time.Since(start) > 2*time.Second {
		t.Fatalf("potential deadlock situation on rotation logic")
	}
}

func newMockCapture(src capture.SourceZeroCopy) *Capture {
	return &Capture{
		iface:         src.Link().Name,
		capLock:       newCaptureLock(),
		flowLog:       capturetypes.NewFlowLog(),
		errMap:        make(map[string]int),
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
