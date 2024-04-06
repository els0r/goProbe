//go:build !slimcap_nomock
// +build !slimcap_nomock

package e2etest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goprobe/writeout"
	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/fako1024/slimcap/capture/pcap"
	"github.com/fako1024/slimcap/link"
	"github.com/stretchr/testify/require"

	slimcap "github.com/fako1024/slimcap/capture"
)

const benchtime = 30 * time.Second

var benchBuf = bytes.NewBuffer(nil)

func TestBenchmarkCaptureThroughput(t *testing.T) {

	if testing.Short() {
		t.SkipNow()
	}

	t.Run("pcap_data", func(t *testing.T) {
		runBenchmarkCaptureThroughput(t, benchBuf, benchtime, setupDataUnblockingSource(t))
	})

	t.Run("random", func(t *testing.T) {
		runBenchmarkCaptureThroughput(t, benchBuf, benchtime, setupSyntheticUnblockingSource(t, true, false))
	})
	t.Run("random+return", func(t *testing.T) {
		runBenchmarkCaptureThroughput(t, benchBuf, benchtime, setupSyntheticUnblockingSource(t, true, true))
	})

	t.Run("non-random", func(t *testing.T) {
		runBenchmarkCaptureThroughput(t, benchBuf, benchtime, setupSyntheticUnblockingSource(t, false, false))
	})
	t.Run("non-random+return", func(t *testing.T) {
		runBenchmarkCaptureThroughput(t, benchBuf, benchtime, setupSyntheticUnblockingSource(t, false, true))
	})

	// Reset all Prometheus counters for the next E2E test to avoid double counting
	capture.ResetCountersTestingOnly()
}

func runBenchmarkCaptureThroughput(t *testing.T, w io.Writer, runtime time.Duration, fn func(*capture.Capture) (capture.Source, error)) {

	// Setup a temporary directory for the test DB
	tempDir, err := os.MkdirTemp(os.TempDir(), "goprobe_e2e_bench")
	if err != nil {
		panic(err)
	}

	// We quit on encountering SIGUSR2 (instead of the ususal SIGTERM or SIGINT)
	// to avoid killing the test
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGUSR2)
	defer stop()

	writeoutHandler := writeout.NewGoDBHandler(tempDir, encoders.EncoderTypeLZ4).
		WithPermissions(goDB.DefaultPermissions)

	captureManager := capture.NewManager(writeoutHandler, capture.WithSourceInitFn(fn))
	_, _, _, err = captureManager.Update(ctx, config.Ifaces{
		"mock": defaultCaptureConfig,
	})
	require.Nil(t, err)

	go func() {
		time.Sleep(runtime)
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGUSR2); err != nil {
			panic(err)
		}
	}()

	// listen for the interrupt signal
	<-ctx.Done()

	// Extract interface stats and calculate metrics
	stats := captureManager.Status(context.Background(), "mock")
	rate := time.Duration(0)
	if stats["mock"].ProcessedTotal > 0 {
		rate = runtime / time.Duration(stats["mock"].ProcessedTotal)
	}

	fmt.Fprintf(w, "%s\t%d\t%d ns/op\n", strings.TrimPrefix(t.Name(), "Test"),
		stats["mock"].ProcessedTotal,
		rate.Nanoseconds())

	fmt.Printf("Packets processed after %v: %d (%v/pkt), %d dropped\n", runtime,
		stats["mock"].ProcessedTotal,
		rate,
		stats["mock"].Dropped)

	shutDownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	captureManager.Close(shutDownCtx)
	cancel()
}

func setupSyntheticUnblockingSource(t testing.TB, randomize, addReturn bool) func(c *capture.Capture) (capture.Source, error) {
	return func(c *capture.Capture) (capture.Source, error) {

		mockSrc, err := afring.NewMockSourceNoDrain(c.Iface(),
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

				if addReturn {
					pRet, err := slimcap.BuildPacket(
						net.ParseIP(fmt.Sprintf("4.5.6.%d", count%254+1)),
						net.ParseIP(fmt.Sprintf("1.2.3.%d", (count+1)%254+1)),
						uint16(count+1)%65535,
						uint16(count)%65535,
						6, []byte{byte(count), byte(count)}, byte(count)%5, count+1)
					require.Nil(t, err)

					require.Nil(t, mockSrc.AddPacket(pRet))
					count++
				}
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
			pRet, err := slimcap.BuildPacket(
				dstIP,
				srcIP,
				80,
				55555,
				6, []byte{1, 2, 3, 4}, slimcap.PacketThisHost, 128)
			require.Nil(t, err)

			for {
				if !mockSrc.CanAddPackets() {
					break
				}
				require.Nil(t, mockSrc.AddPacket(p))
				if addReturn {
					if !mockSrc.CanAddPackets() {
						break
					}
					require.Nil(t, mockSrc.AddPacket(pRet))
				}
			}
		}

		_, err = mockSrc.Run(time.Microsecond)

		return mockSrc, err
	}
}

func setupDataUnblockingSource(t testing.TB) func(c *capture.Capture) (capture.Source, error) {
	return func(c *capture.Capture) (capture.Source, error) {

		pcapData, err := pcaps.ReadFile(filepath.Join(testDataPath, defaultPcapTestFile))
		require.Nil(t, err)

		src, err := pcap.NewSource("default_data", bytes.NewBuffer(pcapData))
		require.Nil(t, err)

		mockSrc, err := afring.NewMockSourceNoDrain(c.Iface(),
			afring.CaptureLength(link.CaptureLengthMinimalIPv6Transport),
			afring.Promiscuous(false),
			afring.BufferSize(1024*1024, 4),
		)
		if err != nil {
			return nil, err
		}

		for mockSrc.CanAddPackets() {
			require.Nil(t, mockSrc.AddPacketFromSource(src))
		}

		_, err = mockSrc.Run(time.Microsecond)

		return mockSrc, err
	}
}
