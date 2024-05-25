package capture

import (
	"net"
	"testing"

	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/fako1024/slimcap/capture"
	"github.com/stretchr/testify/require"
)

func TestBuffer(t *testing.T) {

	localBuf := NewLocalBuffer(testLocalBufferPool)
	data := make([]byte, 128*1024*1024)
	localBuf.Assign(data)

	pkt, err := capture.BuildPacket(
		net.ParseIP("1.2.3.4"),
		net.ParseIP("4.5.6.7"),
		1,
		2,
		17, []byte{1, 2}, capture.PacketOutgoing, 128)

	require.Nil(t, err)
	epHash, auxInfo, errno := ParsePacketV4(pkt.IPLayer())
	count := 0

	t.Run("fill", func(t *testing.T) {
		require.Zero(t, localBuf.Usage())
		for {
			ok := localBuf.Add(epHash[:], pkt.Type(), pkt.TotalLen(), true, auxInfo, errno)
			if !ok {
				break
			}
			count++
		}

		require.False(t, localBuf.Add(epHash[:], pkt.Type(), pkt.TotalLen(), true, auxInfo, errno))
		require.Greater(t, localBuf.Usage(), 0.999)
	})

	t.Run("drain", func(t *testing.T) {

		countDrain := 0
		for {
			dummyAssignHash, dummyPktType, dummyPktSize, dummyIsIPv4, dummyAuxInfo, dummyErrno, dummyOK := localBuf.Next()
			if !dummyOK {
				require.Equal(t, count, countDrain)
				require.Nil(t, dummyAssignHash)

				break
			}

			countDrain++

			_ = dummyAssignHash
			_ = dummyPktType
			_ = dummyPktSize
			_ = dummyIsIPv4
			_ = dummyAuxInfo
			_ = dummyErrno
		}
	})
}

func BenchmarkBuffer(b *testing.B) {

	// benchmaark-level variables to prevent compiler optimizations to
	// make the benchmarks useless
	var (
		dummyAssignHash []byte
		dummyPktType    byte
		dummyPktSize    uint32
		dummyIsIPv4     bool
		dummyAuxInfo    byte
		dummyErrno      capturetypes.ParsingErrno
		dummyOK         bool
	)

	localBuf := new(LocalBuffer)
	data := make([]byte, 128*1024*1024)
	localBuf.Assign(data)

	pkt, err := capture.BuildPacket(
		net.ParseIP("1.2.3.4"),
		net.ParseIP("4.5.6.7"),
		1,
		2,
		17, []byte{1, 2}, capture.PacketOutgoing, 128)

	require.Nil(b, err)
	epHash, auxInfo, errno := ParsePacketV4(pkt.IPLayer())

	b.Run("fill", func(b *testing.B) {

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			if ok := localBuf.Add(epHash[:], pkt.Type(), pkt.TotalLen(), true, auxInfo, errno); !ok {
				localBuf.writeBufPos = 0 // hard reset to provide an "infinite" buffer
			}
		}
	})

	// Fill up the buffer for the next step
	for localBuf.Add(epHash[:], pkt.Type(), pkt.TotalLen(), true, auxInfo, errno) {
	}

	b.Run("drain", func(b *testing.B) {

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			dummyAssignHash, dummyPktType, dummyPktSize, dummyIsIPv4, dummyAuxInfo, dummyErrno, dummyOK = localBuf.Next()
			if !dummyOK {
				localBuf.readBufPos = 0 // hard reset to provide an "infinite" buffer
			}

			_ = dummyAssignHash
			_ = dummyPktType
			_ = dummyPktSize
			_ = dummyIsIPv4
			_ = dummyAuxInfo
			_ = dummyErrno
		}
	})
}
