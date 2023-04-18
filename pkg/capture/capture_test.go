package capture

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/fako1024/slimcap/capture"
	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/fako1024/slimcap/link"
	"github.com/stretchr/testify/require"
)

func TestLowTrafficDeadlock(t *testing.T) {
	testDeadlock(t, 0)
	testDeadlock(t, 1)
	testDeadlock(t, 10)
}

func TestHighTrafficDeadlock(t *testing.T) {
	testDeadlock(t, -1)
}

func TestMockPacketCapturePerformance(t *testing.T) {

	if testing.Short() {
		t.SkipNow()
	}

	testPacket, err := genDummyPacket()
	require.Nil(t, err)

	mockSrc, err := afring.NewMockSource("mock",
		afring.CaptureLength(link.CaptureLengthMinimalIPv4Transport),
	)
	require.Nil(t, err)
	mockC := newMockCapture(mockSrc)

	for mockSrc.CanAddPackets() {
		mockSrc.AddPacket(testPacket)
	}
	mockSrc.RunNoDrain(time.Microsecond)

	runtime := 10 * time.Second
	time.AfterFunc(runtime, func() {
		require.Nil(t, mockSrc.Close())
		require.Nil(t, mockSrc.Free())
	})

	mockC.process()
	for _, v := range mockC.flowLog.flowMap {
		fmt.Printf("Packets processed after %v: %d (%v/pkt)\n", runtime, v.packetsSent, runtime/time.Duration(v.packetsSent))
	}
}

func testDeadlock(t *testing.T, maxPkts int) {

	mockSrc, err := afring.NewMockSource("mock",
		afring.CaptureLength(link.CaptureLengthMinimalIPv4Transport),
	)
	require.Nil(t, err)
	mockC := newMockCapture(mockSrc)

	testPacket, err := genDummyPacket()
	require.Nil(t, err)

	var errChan chan error
	if maxPkts >= 0 {
		go func() {
			errChan = mockSrc.Run()
			for i := 0; i < maxPkts; i++ {
				mockSrc.AddPacket(testPacket)
			}
			mockSrc.Done()
		}()
	} else {
		for mockSrc.CanAddPackets() {
			mockSrc.AddPacket(testPacket)
		}
		errChan = mockSrc.RunNoDrain(time.Microsecond)
	}

	start := time.Now()
	time.AfterFunc(100*time.Millisecond, func() {
		for i := 0; i < 20; i++ {
			mockC.rotate()
			time.Sleep(10 * time.Millisecond)
		}

		require.Nil(t, mockSrc.Close())
	})

	mockC.process()

	select {
	case <-errChan:
		break
	case <-time.After(10 * time.Second):
		t.Fatalf("potential deadlock situation on rotation logic")
	}

	if time.Since(start) > 2*time.Second {
		t.Fatalf("potential deadlock situation on rotation logic")
	}

	require.Nil(t, mockSrc.Free())
}

func newMockCapture(src capture.SourceZeroCopy) *Capture {
	return &Capture{
		iface:         src.Link().Name,
		mutex:         sync.Mutex{},
		cmdChan:       make(chan captureCommand),
		captureErrors: make(chan error),
		lastRotationStats: Stats{
			CaptureStats: &CaptureStats{},
		},
		rotationState: newRotationState(),
		flowLog:       NewFlowLog(),
		errMap:        make(map[string]int),
		ctx:           context.Background(),
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
