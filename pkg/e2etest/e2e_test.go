package e2etest

import (
	"bytes"
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/cmd/goQuery/cmd"

	// "github.com/els0r/goProbe/cmd/goQuery/commands"

	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"

	slimcap "github.com/fako1024/slimcap/capture"
	"github.com/fako1024/slimcap/capture/afpacket/afring"
	"github.com/fako1024/slimcap/capture/pcap"
	"github.com/fako1024/slimcap/link"
)

const (
	testDataPath        = "testdata"
	defaultPcapTestFile = "default.pcap.gz"
)

//go:embed testdata/*
var pcaps embed.FS

var defaultCaptureConfig = config.CaptureConfig{
	Promisc: false,
	RingBuffer: &config.RingBufferConfig{
		BlockSize: 1048576,
		NumBlocks: 4,
	},
}

var externalPCAPPath string

func TestStartStop(t *testing.T) {
	for i := 0; i < 1000; i++ {
		testStartStop(t)
	}
}

func testStartStop(t *testing.T) {

	// Setup a temporary directory for the test DB
	tempDir, err := os.MkdirTemp(os.TempDir(), "goprobe_e2e_startstop")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	// We quit on encountering SIGUSR2 (instead of the ususal SIGTERM or SIGINT)
	// to avoid killing the test
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGUSR2)
	defer stop()

	ifaces := config.Ifaces{
		"mock1": defaultCaptureConfig,
		"mock2": defaultCaptureConfig,
	}

	captureManager, err := capture.InitManager(ctx, &config.Config{
		DB: config.DBConfig{
			Path:        tempDir,
			EncoderType: encoders.EncoderTypeLZ4.String(),
			Permissions: goDB.DefaultPermissions,
		},
		Interfaces: ifaces,
		Logging: config.LogConfig{
			Destination: "logfmt",
			Level:       "warn",
		},
	}, capture.WithSourceInitFn(func(c *capture.Capture) (slimcap.SourceZeroCopy, error) {
		mockSrc, err := afring.NewMockSource(c.Iface(),
			afring.CaptureLength(link.CaptureLengthMinimalIPv6Transport),
			afring.Promiscuous(false),
			afring.BufferSize(1024*1024, 4),
		)
		require.Nil(t, err)
		return mockSrc, nil
	}), capture.WithSkipWriteoutSchedule(true))
	require.Nil(t, err)

	// Wait until goProbe is done processing all packets, then kill it in the
	// background via the SIGUSR2 signal
	// Send the termination signal to goProbe
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGUSR2); err != nil {
		panic(err)
	}

	// Wait for the interrupt signal
	<-ctx.Done()

	// Finish up
	shutDownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	captureManager.Close(shutDownCtx)

	cancel()
}

func TestE2EBasic(t *testing.T) {
	pcapData, err := pcaps.ReadFile(filepath.Join(testDataPath, defaultPcapTestFile))
	require.Nil(t, err)

	testE2E(t, pcapData)
}

func TestE2EMultipleIfaces(t *testing.T) {
	pcapData, err := pcaps.ReadFile(filepath.Join(testDataPath, defaultPcapTestFile))
	require.Nil(t, err)

	for _, n := range []int{
		2, 3, 5, 10, 21, 100,
	} {
		t.Run(fmt.Sprintf("%02d interfaces", n), func(t *testing.T) {

			// Use identical data several times
			ifaceData := make([][]byte, n)
			for i := 0; i < len(ifaceData); i++ {
				ifaceData[i] = pcapData
			}

			testE2E(t, ifaceData...)
		})
	}
}

func TestE2EExtended(t *testing.T) {
	pcapDir, err := pcaps.ReadDir(testDataPath)
	require.Nil(t, err)

	for _, dirent := range pcapDir {
		path := filepath.Join(testDataPath, dirent.Name())

		t.Run(path, func(t *testing.T) {
			pcapData, err := pcaps.ReadFile(path)
			require.Nil(t, err)

			testE2E(t, pcapData)
		})
	}
}

func TestE2EExtendedPermuted(t *testing.T) {
	pcapDir, err := pcaps.ReadDir(testDataPath)
	require.Nil(t, err)

	Perm(pcapDir, func(de []fs.DirEntry) {
		ifaceData := make([][]byte, 0)

		var testMsg string
		for _, dirent := range de {
			path := filepath.Join(testDataPath, dirent.Name())
			testMsg += " " + path

			pcapData, err := pcaps.ReadFile(path)
			require.Nil(t, err)
			ifaceData = append(ifaceData, pcapData)
		}

		t.Run(testMsg, func(t *testing.T) {
			testE2E(t, ifaceData...)
		})
	})
}

func TestE2EExternal(t *testing.T) {
	if externalPCAPPath == "" {
		t.SkipNow()
	}

	stat, err := os.Stat(externalPCAPPath)
	require.Nil(t, err)
	if stat.IsDir() {

		require.Nil(t, filepath.WalkDir(externalPCAPPath, func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}
			if err != nil {
				return err
			}

			fmt.Println("Running E2E test on", path)
			pcapData, err := os.ReadFile(filepath.Clean(path))
			require.Nil(t, err)

			testE2E(t, pcapData)

			return nil
		}))

	} else {
		fmt.Println("Running E2E test on", externalPCAPPath)
		pcapData, err := os.ReadFile(filepath.Clean(externalPCAPPath))
		require.Nil(t, err)

		testE2E(t, pcapData)
	}
}

func testE2E(t *testing.T, datasets ...[]byte) {

	// Setup a temporary directory for the test DB
	tempDir, err := os.MkdirTemp(os.TempDir(), "goprobe_e2e")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	// Define mock interfaces
	var mockIfaces mockIfaces
	for i, data := range datasets {
		mockIfaces = append(mockIfaces, newPcapSource(t, fmt.Sprintf("mock%03d", i+1), data))
	}

	// Run GoProbe
	runGoProbe(t, tempDir, setupSources(t, mockIfaces))

	resGoQueryList := make([]goDB.InterfaceMetadata, 0)
	runGoQuery(t, &resGoQueryList, []string{
		"-e", "json",
		"-l", time.Now().Add(time.Hour).Format(time.ANSIC),
		"-d", tempDir,
		"list",
	})

	// Run GoQuery and build reference results from tracking
	resGoQuery := new(results.Result)
	runGoQuery(t, resGoQuery, []string{
		"-i", strings.Join(mockIfaces.Names(), ","),
		"-e", "json",
		"-l", time.Now().Add(time.Hour).Format(time.ANSIC),
		"-d", tempDir,
		"-n", strconv.Itoa(100000),
		"-s", "packets",
		"sip,dip,dport,proto",
	})
	resReference, listReference := mockIfaces.BuildResults(t, tempDir, resGoQuery)

	// Counter consistency checks
	require.Equal(t, mockIfaces.NProcessed(), resGoQuery.Summary.Totals.PacketsRcvd)
	require.Equal(t, mockIfaces.NProcessed(), mockIfaces.NRead()-mockIfaces.NErr())

	// List target consistency check (do not fail yet to show details in the next check)
	if !reflect.DeepEqual(listReference, resGoQueryList) {
		t.Errorf("Mismatch on goQuery list target, want %+v, have %+v", listReference, resGoQueryList)
	}

	// Summary consistency check (do not fail yet to show details in the next check)
	if !reflect.DeepEqual(resReference.Summary, resGoQuery.Summary) {
		t.Errorf("Mismatch on goQuery summary, want %+v, have %+v", resReference.Summary, resGoQuery.Summary)
	}

	// Since testify creates very unreadable output when comparing the struct directly we build a stringified
	// version of the result rows and compare that
	refRows := make([]string, len(resReference.Rows))
	for i := 0; i < len(refRows); i++ {
		refRows[i] = fmt.Sprintf("%s (%s): %s %s",
			resReference.Rows[i].Labels.Hostname, resReference.Rows[i].Labels.HostID,
			resReference.Rows[i].Attributes,
			resReference.Rows[i].Counters)
	}
	resRows := make([]string, len(resGoQuery.Rows))
	for i := 0; i < len(resRows); i++ {
		resRows[i] = fmt.Sprintf("%s (%s): %s %s",
			resGoQuery.Rows[i].Labels.Hostname, resGoQuery.Rows[i].Labels.HostID,
			resGoQuery.Rows[i].Attributes,
			resGoQuery.Rows[i].Counters)
	}
	require.EqualValues(t, refRows, resRows)
}

func runGoProbe(t *testing.T, testDir string, sourceInitFn func() (mockIfaces, func(c *capture.Capture) (slimcap.SourceZeroCopy, error))) {

	// We quit on encountering SIGUSR2 (instead of the ususal SIGTERM or SIGINT)
	// to avoid killing the test
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGUSR2)
	defer stop()

	mockIfaces, initFn := sourceInitFn()

	ifaceConfigs := make(config.Ifaces)
	for _, iface := range mockIfaces {
		ifaceConfigs[iface.name] = defaultCaptureConfig
	}
	captureManager, err := capture.InitManager(ctx, &config.Config{
		DB: config.DBConfig{
			Path:        testDir,
			EncoderType: encoders.EncoderTypeLZ4.String(),
			Permissions: goDB.DefaultPermissions,
		},
		Interfaces: ifaceConfigs,
		Logging:    config.LogConfig{},
	},
		capture.WithSourceInitFn(initFn),
		capture.WithSkipWriteoutSchedule(true),
	)
	require.Nil(t, err)

	// Wait until goProbe is done processing all packets, then kill it in the
	// background via the SIGUSR2 signal
	go mockIfaces.KillGoProbeOnceDone(captureManager)

	// Wait for the interrupt signal
	<-ctx.Done()

	// Finish up
	shutDownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	captureManager.Close(shutDownCtx)
	cancel()
}

func runGoQuery(t *testing.T, res interface{}, args []string) {

	buf := bytes.NewBuffer(nil)
	old := os.Stdout // keep backup of STDOUT
	r, wr, err := os.Pipe()
	require.Nil(t, err)
	os.Stdout = wr

	// copy the output in a separate goroutine so printing can't block indefinitely
	copyDone := make(chan struct{})
	go func() {
		if _, err := io.Copy(buf, r); err != nil {
			panic(err)
		}
		copyDone <- struct{}{}
	}()

	command := cmd.GetRootCmd()
	command.SetArgs(args)

	require.Nil(t, command.Execute())
	require.Nil(t, wr.Close())
	<-copyDone
	os.Stdout = old // restore the inital STDOUT

	require.Nil(t, jsoniter.NewDecoder(buf).Decode(&res))
}

func setupSources(t testing.TB, ifaces mockIfaces) func() (mockIfaces, func(c *capture.Capture) (slimcap.SourceZeroCopy, error)) {

	fnMap := make(map[string]func(c *capture.Capture) (slimcap.SourceZeroCopy, error))
	for _, mockIface := range ifaces {
		fnMap[mockIface.name] = mockIface.sourceInitFn
	}

	return func() (mockIfaces, func(c *capture.Capture) (slimcap.SourceZeroCopy, error)) {
		return ifaces, func(c *capture.Capture) (slimcap.SourceZeroCopy, error) {
			mockIfaceFn, ok := fnMap[c.Iface()]
			if !ok {
				return nil, fmt.Errorf("unable to find interface `%s` in list of mock interfaces", c.Iface())
			}

			return mockIfaceFn(c)
		}
	}
}

func newPcapSource(t testing.TB, name string, data []byte) (res *mockIface) {

	res = &mockIface{
		name: name,
		src:  &afring.MockSource{},
		tracking: &mockTracking{
			done: make(chan struct{}, 1),
		},
		flows:   &map[capturetypes.EPHash]types.Counters{},
		RWMutex: sync.RWMutex{},
	}

	res.sourceInitFn = func(c *capture.Capture) (slimcap.SourceZeroCopy, error) {

		res.Lock()
		defer res.Unlock()

		src, err := pcap.NewSource(res.name, bytes.NewBuffer(data))
		require.Nil(t, err)
		src.PacketAddCallbackFn(func(payload []byte, totalLen uint32, pktType, ipLayerOffset byte) {

			res.Lock()
			defer res.Unlock()

			res.tracking.nRead++
		})

		mockSrc, err := afring.NewMockSource(c.Iface(),
			afring.CaptureLength(link.CaptureLengthMinimalIPv6Transport),
			afring.Promiscuous(false),
			afring.BufferSize(1024*1024, 4),
		)
		require.Nil(t, err)
		*res.src = *mockSrc

		pkt := mockSrc.NewPacket()
		mockSrc.PacketAddCallbackFn(func(payload []byte, totalLen uint32, pktType, ipLayerOffset byte) {

			res.Lock()
			defer res.Unlock()

			pkt = slimcap.NewIPPacket(pkt, payload, pktType, int(totalLen), ipLayerOffset)
			hash, isIPv4, auxInfo, err := capture.ParsePacket(pkt.IPLayer(), pkt.TotalLen())
			if err != nil {
				res.tracking.nErr++
				return
			}

			hashReverse := hash.Reverse()
			if direction := capturetypes.ClassifyPacketDirection(hash, isIPv4, auxInfo); direction != capturetypes.DirectionUnknown {
				if direction == capturetypes.DirectionReverts || direction == capturetypes.DirectionMaybeReverts {
					hash, hashReverse = hashReverse, hash
				}
			}

			hash[34], hash[35] = 0, 0
			hashReverse[34], hashReverse[35] = 0, 0

			if flow, exists := (*res.flows)[hash]; exists {
				if pkt.Type() != slimcap.PacketOutgoing {
					(*res.flows)[hash] = flow.Add(types.Counters{
						PacketsRcvd: 1,
						BytesRcvd:   uint64(totalLen),
					})
				} else {
					(*res.flows)[hash] = flow.Add(types.Counters{
						PacketsSent: 1,
						BytesSent:   uint64(totalLen),
					})
				}
			} else if flow, exists = (*res.flows)[hashReverse]; exists {
				if pkt.Type() != slimcap.PacketOutgoing {
					(*res.flows)[hashReverse] = flow.Add(types.Counters{
						PacketsRcvd: 1,
						BytesRcvd:   uint64(totalLen),
					})
				} else {
					(*res.flows)[hashReverse] = flow.Add(types.Counters{
						PacketsSent: 1,
						BytesSent:   uint64(totalLen),
					})
				}
			} else {
				if pkt.Type() != slimcap.PacketOutgoing {
					(*res.flows)[hash] = types.Counters{
						PacketsRcvd: 1,
						BytesRcvd:   uint64(totalLen),
					}
				} else {
					(*res.flows)[hash] = types.Counters{
						PacketsSent: 1,
						BytesSent:   uint64(totalLen),
					}
				}
			}

			res.tracking.nProcessed++
		})

		mockSrc.Pipe(src, res.tracking.done)

		return mockSrc, nil
	}

	return
}

func newSyntheticSource(t testing.TB, name string, nPkts int) (res *mockIface) {

	res = &mockIface{
		name:     name,
		src:      &afring.MockSource{},
		tracking: &mockTracking{},
		flows:    &map[capturetypes.EPHash]types.Counters{},
		RWMutex:  sync.RWMutex{},
	}

	res.sourceInitFn = func(c *capture.Capture) (slimcap.SourceZeroCopy, error) {

		res.Lock()
		defer res.Unlock()

		mockSrc, err := afring.NewMockSource(c.Iface(),
			afring.CaptureLength(link.CaptureLengthMinimalIPv6Transport),
			afring.Promiscuous(false),
			afring.BufferSize(1024*1024, 4),
		)
		if err != nil {
			return nil, err
		}
		*res.src = *mockSrc

		mockSrc.PacketAddCallbackFn(func(payload []byte, totalLen uint32, pktType, ipLayerOffset byte) {

			res.Lock()
			defer res.Unlock()

			pkt := slimcap.NewIPPacket(nil, payload, pktType, int(totalLen), ipLayerOffset)
			hash, isIPv4, auxInfo, err := capture.ParsePacket(pkt.IPLayer(), pkt.TotalLen())
			if err != nil {
				res.tracking.nErr++
				return
			}

			hashReverse := hash.Reverse()
			if direction := capturetypes.ClassifyPacketDirection(hash, isIPv4, auxInfo); direction != capturetypes.DirectionUnknown {
				if direction == capturetypes.DirectionReverts || direction == capturetypes.DirectionMaybeReverts {
					hash, hashReverse = hashReverse, hash
				}
			}

			hash[34], hash[35] = 0, 0
			hashReverse[34], hashReverse[35] = 0, 0

			if flow, exists := (*res.flows)[hash]; exists {
				if pkt.Type() != slimcap.PacketOutgoing {
					(*res.flows)[hash] = flow.Add(types.Counters{
						PacketsRcvd: 1,
						BytesRcvd:   uint64(totalLen),
					})
				} else {
					(*res.flows)[hash] = flow.Add(types.Counters{
						PacketsSent: 1,
						BytesSent:   uint64(totalLen),
					})
				}
			} else if flow, exists = (*res.flows)[hashReverse]; exists {
				if pkt.Type() != slimcap.PacketOutgoing {
					(*res.flows)[hashReverse] = flow.Add(types.Counters{
						PacketsRcvd: 1,
						BytesRcvd:   uint64(totalLen),
					})
				} else {
					(*res.flows)[hashReverse] = flow.Add(types.Counters{
						PacketsSent: 1,
						BytesSent:   uint64(totalLen),
					})
				}
			} else {
				if pkt.Type() != slimcap.PacketOutgoing {
					(*res.flows)[hash] = types.Counters{
						PacketsRcvd: 1,
						BytesRcvd:   uint64(totalLen),
					}
				} else {
					(*res.flows)[hash] = types.Counters{
						PacketsSent: 1,
						BytesSent:   uint64(totalLen),
					}
				}
			}

			res.tracking.nProcessed++
		})

		mockSrc.Run()
		var n = uint16(nPkts)
		go func() {
			for i := uint16(1); i <= n; i++ {
				for j := uint16(1); j <= n; j++ {

					p, err := slimcap.BuildPacket(
						net.ParseIP(fmt.Sprintf("1.2.3.%d", i%254+1)),
						net.ParseIP(fmt.Sprintf("4.5.6.%d", j%254+1)),
						i,
						j,
						17, []byte{byte(i), byte(j)}, byte(i+j)%5, int(i+j))
					require.Nil(t, err)

					require.Nil(t, mockSrc.AddPacket(p))
				}
			}
			mockSrc.FinalizeBlock(false)
			mockSrc.Done()
		}()

		return mockSrc, nil
	}

	return
}

func TestMain(m *testing.M) {

	flag.StringVar(&externalPCAPPath, "ext-pcap-data", "", "path to external pcap file(s) for E2E tests (can be a single file or directory)")
	flag.Parse()

	os.Exit(m.Run())
}
