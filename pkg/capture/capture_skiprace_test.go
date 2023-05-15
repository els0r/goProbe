//go:build !race

package capture

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/fako1024/slimcap/link"
	"github.com/stretchr/testify/require"
)

func TestLowTrafficDeadlock(t *testing.T) {
	for _, n := range []int{0, 1, 10} {
		t.Run(fmt.Sprintf("%d packets", n), func(t *testing.T) {
			testDeadlock(t, n)
		})
	}
}

func TestHighTrafficDeadlock(t *testing.T) {
	testDeadlock(t, -1)
}

func TestMockPacketCapturePerformance(t *testing.T) {

	ctx := context.Background()

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
		require.Nil(t, mockSrc.AddPacket(testPacket))
	}
	mockSrc.RunNoDrain(time.Microsecond)

	runtime := 10 * time.Second
	time.AfterFunc(runtime, func() {
		require.Nil(t, mockC.close())
	})

	mockC.process(ctx)

	mockC.lock()
	flows := mockC.flowLog.Flows()
	for _, v := range flows {
		fmt.Printf("Packets processed after %v: %d (%v/pkt)\n", runtime, v.PacketsSent(), runtime/time.Duration(v.PacketsSent()))
	}
	mockC.unlock()
}
