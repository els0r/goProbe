//go:build !race

package capture

import (
	"fmt"
	"testing"
	"time"

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
