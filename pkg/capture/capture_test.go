package capture

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/fako1024/slimcap/capture"
	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/fako1024/slimcap/link"
	"github.com/stretchr/testify/require"
)

func testDeadlock(t *testing.T, maxPkts int) {

	ctx := context.Background()
	mockSrc, err := afring.NewMockSource("mock",
		afring.CaptureLength(link.CaptureLengthMinimalIPv4Transport),
	)
	require.Nil(t, err)
	mockC := newMockCapture(mockSrc)

	testPacket, err := genDummyPacket()
	require.Nil(t, err)

	var errChan chan error
	if maxPkts >= 0 {

		// TODO: Remove race condition when filling the buffer in a goroutine
		// go func() {
		errChan = mockSrc.Run()
		for i := 0; i < maxPkts; i++ {
			require.Nil(t, mockSrc.AddPacket(testPacket))
		}
		mockSrc.Done()
		mockSrc.ForceBlockRelease()
		// }()
	} else {
		for mockSrc.CanAddPackets() {
			require.Nil(t, mockSrc.AddPacket(testPacket))
		}
		errChan = mockSrc.RunNoDrain(time.Microsecond)
	}

	procErrChan := mockC.process(ctx)

	start := time.Now()
	time.AfterFunc(100*time.Millisecond, func() {
		for i := 0; i < 20; i++ {
			mockC.rotate(ctx)
			time.Sleep(10 * time.Millisecond)
		}

		require.Nil(t, mockC.Close())
	})

	select {
	case err := <-procErrChan:
		require.Nil(t, err)
	case <-errChan:
		break
	case <-time.After(10 * time.Second):
		t.Fatalf("potential deadlock situation on rotation logic")
	}

	if time.Since(start) > 2*time.Second {
		t.Fatalf("potential deadlock situation on rotation logic")
	}
}

func newMockCapture(src capture.SourceZeroCopy) *Capture {
	return &Capture{
		iface:         src.Link().Name,
		lock:          newCaptureLock(),
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
