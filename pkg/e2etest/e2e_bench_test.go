//go:build !race

package e2etest

import (
	"context"
	"fmt"
	"net"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goprobe/writeout"
	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/fako1024/slimcap/link"
	"github.com/stretchr/testify/require"

	slimcap "github.com/fako1024/slimcap/capture"
)

func TestBenchmarkCaptureThroughput(t *testing.T) {

	if testing.Short() {
		t.SkipNow()
	}

	t.Run("random", func(t *testing.T) {
		runBenchmarkCaptureThroughput(t, 10*time.Second, true)
	})

	t.Run("non-random", func(t *testing.T) {
		runBenchmarkCaptureThroughput(t, 10*time.Second, false)
	})
}

func runBenchmarkCaptureThroughput(t *testing.T, runtime time.Duration, randomize bool) {

	// We quit on encountering SIGTERM or SIGINT (see further down)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGUSR2)
	defer stop()
	captureManager := capture.NewManager(ctx).SetSourceInitFn(setupSyntheticUnblockingSource(t, randomize))
	captureManager.Update(config.Ifaces{
		"mock": defaultCaptureConfig,
	}, nil)

	// start goroutine for writeouts
	writeoutHandler := writeout.NewHandler(captureManager, encoders.EncoderTypeLZ4).
		WithPermissions(goDB.DefaultPermissions)

	// start writeout handler
	writeoutHandler.HandleWriteouts()

	// start regular rotations
	// writeoutHandler.HandleRotations(ctx, time.Hour)
	go func() {
		time.Sleep(runtime)
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGUSR2); err != nil {
			panic(err)
		}
	}()

	// listen for the interrupt signal, then flush data
	<-ctx.Done()
	require.Nil(t, writeoutHandler.FullWriteout(context.Background(), time.Now()))

	stats := captureManager.Status()["mock"]
	fmt.Printf("Packets processed after %v: %d (%v/pkt), %d dropped\n", runtime, stats.PacketStats.PacketsCapturedOverall, runtime/time.Duration(stats.PacketStats.PacketsCapturedOverall), stats.PacketStats.Dropped)

	writeoutHandler.Close()
	captureManager.CloseAll()
}

func setupSyntheticUnblockingSource(t testing.TB, randomize bool) func(c *capture.Capture) (slimcap.Source, error) {
	return func(c *capture.Capture) (slimcap.Source, error) {

		mockSrc, err := afring.NewMockSource(c.Iface(),
			afring.CaptureLength(link.CaptureLengthMinimalIPv6Transport),
			afring.Promiscuous(false),
			afring.BufferSize(1024*1024, 4),
		)
		if err != nil {
			return nil, err
		}

		if randomize {
			count := 0
			for mockSrc.CanAddPackets() {
				p, err := slimcap.BuildPacket(
					net.ParseIP(fmt.Sprintf("1.2.3.%d", (count+1)%254+1)),
					net.ParseIP(fmt.Sprintf("4.5.6.%d", count%254+1)),
					uint16(count)%65535,
					uint16(count+1)%65535,
					6, []byte{byte(count), byte(count)}, byte(count)%5, count+1)
				require.Nil(t, err)

				require.Nil(t, mockSrc.AddPacket(p))
				count++
			}
		} else {
			srcIP, dstIP := net.ParseIP("1.2.3.4"), net.ParseIP("4.5.6.7")
			p, err := slimcap.BuildPacket(
				srcIP,
				dstIP,
				55555,
				80,
				6, []byte{1, 2, 3, 4}, slimcap.PacketOutgoing, 128)
			require.Nil(t, err)

			for mockSrc.CanAddPackets() {
				require.Nil(t, mockSrc.AddPacket(p))
			}
		}

		mockSrc.RunNoDrain(time.Microsecond)

		return mockSrc, nil
	}
}
