package capture

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"testing"

	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/fako1024/slimcap/capture"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type testParams struct {
	sip, dip          string
	sport, dport      uint16
	proto             byte
	AuxInfo           byte
	expectedDirection capturetypes.Direction
}

func (p testParams) String() string {
	return fmt.Sprintf("%s:%d->%s:%d_%d_%x", p.sip, p.sport, p.dip, p.dport, p.proto, p.AuxInfo)
}

type testCase struct {
	input  testParams
	EPHash capturetypes.EPHash
	IsIPv4 bool
}

var testCases = []testParams{
	{"2c04:4000::6ab", "2c01:2000::3", 0, 0, capturetypes.ICMPv6, 0x80, capturetypes.DirectionRemains},       // capturetypes.ICMPv6 echo request
	{"2c01:2000::3", "2c04:4000::6ab", 0, 0, capturetypes.ICMPv6, 0x81, capturetypes.DirectionReverts},       // capturetypes.ICMPv6 echo reply
	{"fe80::3df3:abbf:3d8d:7f03", "ff02::2", 0, 0, capturetypes.ICMPv6, 0x86, capturetypes.DirectionRemains}, // capturetypes.ICMPv6 router advertisement
	{"10.0.0.1", "10.0.0.2", 0, 0, capturetypes.ICMP, 0x08, capturetypes.DirectionRemains},                   // capturetypes.ICMP echo request
	{"10.0.0.2", "10.0.0.1", 0, 0, capturetypes.ICMP, 0x00, capturetypes.DirectionReverts},                   // capturetypes.ICMP echo reply
	{"10.0.0.1", "10.0.0.2", 37485, 17500, capturetypes.TCP, 0, capturetypes.DirectionRemains},               // TCP request to Dropbox LanSync from client on ephemeral port
	{"10.0.0.2", "10.0.0.1", 17500, 34000, capturetypes.TCP, 0, capturetypes.DirectionReverts},               // TCP response from Dropbox LanSync to client on ephemeral port
	{"2c04:4000::6ab", "2c01:2000::3", 37485, 17500, capturetypes.TCP, 0, capturetypes.DirectionRemains},     // TCP request to Dropbox LanSync from client on ephemeral port
	{"2c01:2000::3", "2c04:4000::6ab", 17500, 34000, capturetypes.TCP, 0, capturetypes.DirectionReverts},     // TCP response from Dropbox LanSync to client on ephemeral port
	{"10.0.0.1", "4.5.6.7", 33561, 444, capturetypes.UDP, 0, capturetypes.DirectionRemains},                  // capturetypes.UDP request from ephemaral port to privileged port
	{"4.5.6.7", "10.0.0.1", 444, 33561, capturetypes.UDP, 0, capturetypes.DirectionReverts},                  // capturetypes.UDP response from privileged port to ephemaral port
	{"10.0.0.1", "4.5.6.7", 33561, 33560, capturetypes.UDP, 0, capturetypes.DirectionRemains},                // capturetypes.UDP request from higher ephemaral port to lower ephemaral port
	{"4.5.6.7", "10.0.0.1", 33560, 33561, capturetypes.UDP, 0, capturetypes.DirectionReverts},                // capturetypes.UDP response from lower ephemaral port to higher ephemaral port
	{"10.0.0.1", "4.5.6.7", 445, 444, capturetypes.UDP, 0, capturetypes.DirectionRemains},                    // capturetypes.UDP request from higher privileged port to lower privileged port
	{"4.5.6.7", "10.0.0.1", 444, 445, capturetypes.UDP, 0, capturetypes.DirectionReverts},                    // capturetypes.UDP response from lower privileged port to higher privileged port
	{"10.0.0.1", "4.5.6.7", 33561, 33561, capturetypes.UDP, 0, capturetypes.DirectionRemains},                // capturetypes.UDP packet from identical ephemaral ports (no change, have to assume this is the first packet)
	{"10.0.0.1", "4.5.6.7", 444, 444, capturetypes.UDP, 0, capturetypes.DirectionRemains},                    // capturetypes.UDP packet from identical privileged ports (no change, have to assume this is the first packet)
	{"0.0.0.0", "255.255.255.255", 68, 67, capturetypes.UDP, 0, capturetypes.DirectionRemains},               // DHCP broadcast packet
	{"10.0.0.1", "10.0.0.2", 67, 68, capturetypes.UDP, 0, capturetypes.DirectionReverts},                     // DHCP reply (unicast)
	{"10.0.0.1", "4.5.6.7", 0, 53, capturetypes.UDP, 0, capturetypes.DirectionRemains},                       // DNS request from NULLed ephemaral port
	{"10.0.0.1", "4.5.6.7", 0, 53, capturetypes.TCP, 0, capturetypes.DirectionRemains},                       // DNS request from NULLed ephemaral port
	{"10.0.0.1", "4.5.6.7", 0, 80, capturetypes.TCP, 0, capturetypes.DirectionRemains},                       // HTTP request from NULLed ephemaral port
	{"10.0.0.1", "4.5.6.7", 0, 443, capturetypes.TCP, 0, capturetypes.DirectionRemains},                      // HTTPS request from NULLed ephemaral port
	{"2c04:4000::6ab", "2c04:4000::6ab", 33561, 33561, capturetypes.UDP, 0, capturetypes.DirectionRemains},   // capturetypes.UDP packet from identical ephemaral ports (no change, have to assume this is the first packet)
	{"2c04:4000::6ab", "2c04:4000::6ab", 444, 444, capturetypes.UDP, 0, capturetypes.DirectionRemains},       // capturetypes.UDP packet from identical privileged ports (no change, have to assume this is the first packet)
	{"2c04:4000::6ab", "2c04:4000::6ab", 0, 53, capturetypes.UDP, 0, capturetypes.DirectionRemains},          // DNS request from NULLed ephemaral port
}

func TestMaxEphemeralPort(t *testing.T) {
	require.Equal(t, uint16(65535), capturetypes.MaxEphemeralPort, "Maximum ephemeral port is != max(uint16), adapt isEphemeralPort() accordingly !")
}

func TestPortMergeLogic(t *testing.T) {
	for i := uint16(0); i < 65535; i++ {
		if i == 53 || i == 80 || i == 443 {
			require.Truef(t, isCommonPort(uint16ToPort(i), capturetypes.TCP), "Port %d/TCP considered common port, adapt isNotCommonPort() accordingly !", i)
		} else {
			require.Falsef(t, isCommonPort(uint16ToPort(i), capturetypes.TCP), "Port %d/TCP not considered common port, adapt isNotCommonPort() accordingly !", i)
		}
		if i == 53 || i == 443 {
			require.Truef(t, isCommonPort(uint16ToPort(i), capturetypes.UDP), "Port %d/capturetypes.UDP considered common port, adapt isNotCommonPort() accordingly !", i)
		} else {
			require.Falsef(t, isCommonPort(uint16ToPort(i), capturetypes.UDP), "Port %d/capturetypes.UDP not considered common port, adapt isNotCommonPort() accordingly !", i)
		}
	}
}

func TestSmallInvalidIPPackets(t *testing.T) {
	invalidProto := byte(0xF8)

	for _, params := range []testParams{
		{"10.0.0.1", "10.0.0.2", 0, 0, invalidProto, 0x0, capturetypes.DirectionRemains},
		{"2c04:4000::6ab", "2c01:2000::3", 0, 0, invalidProto, 0x0, capturetypes.DirectionRemains},
	} {
		testPacket := params.genDummyPacket(0)
		refHash, refIsIPv4 := params.genEPHash()

		var croppedIPLayer capture.IPLayer
		if refIsIPv4 {
			croppedIPLayer = testPacket.IPLayer()[:ipv4.HeaderLen]
		} else {
			croppedIPLayer = testPacket.IPLayer()[:ipv6.HeaderLen]
		}

		epHash, isIPv4, _, errno := ParsePacket(croppedIPLayer)
		require.Equal(t, capturetypes.ErrnoOK, errno, "population error")

		require.Equal(t, refHash, epHash)
		require.Equal(t, refIsIPv4, isIPv4)
	}
}

func TestPopulation(t *testing.T) {
	for _, params := range testCases {
		t.Run(params.String(), func(t *testing.T) {
			testPacket := params.genDummyPacket(0)
			refHash, refIsIPv4 := params.genEPHash()

			epHash, isIPv4, _, errno := ParsePacket(testPacket.IPLayer())
			require.Equal(t, capturetypes.ErrnoOK, errno, "population error")

			require.Equal(t, refHash, epHash)
			require.Equal(t, refIsIPv4, isIPv4)
		})
	}
}

func TestClassification(t *testing.T) {
	for _, params := range testCases {
		t.Run(params.String(), func(t *testing.T) {
			testCase := params.genTestCase()
			require.Equal(t, params.expectedDirection, capturetypes.ClassifyPacketDirection(testCase.EPHash, testCase.IsIPv4, testCase.input.AuxInfo), "classification mismatch")
		})
	}
}

func BenchmarkPopulation(b *testing.B) {
	for _, params := range testCases {
		b.Run(params.String(), func(b *testing.B) {
			testPacket := params.genDummyPacket(0)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, _, _ = ParsePacket(testPacket.IPLayer())
			}
		})
	}
}

func BenchmarkClassification(b *testing.B) {
	for _, params := range testCases {
		b.Run(params.String(), func(b *testing.B) {
			testCase := params.genTestCase()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				capturetypes.ClassifyPacketDirection(testCase.EPHash, testCase.IsIPv4, testCase.input.AuxInfo)
			}
		})
	}
}

func (p testParams) genTestCase() testCase {
	EPHash, IsIPv4 := p.genEPHash()
	return testCase{
		input:  p,
		EPHash: EPHash,
		IsIPv4: IsIPv4,
	}
}

func (p testParams) genEPHash() (res capturetypes.EPHash, IsIPv4 bool) {

	src, err := netip.ParseAddr(p.sip)
	if err != nil {
		panic(err)
	}
	dst, err := netip.ParseAddr(p.dip)
	if err != nil {
		panic(err)
	}

	IsIPv4 = src.Is4()
	if IsIPv4 {
		tmpSrc, tmpDst := src.As4(), dst.As4()
		copy(res[0:], tmpSrc[:])
		copy(res[16:], tmpDst[:])
	} else {
		tmpSrc, tmpDst := src.As16(), dst.As16()
		copy(res[0:], tmpSrc[:])
		copy(res[16:], tmpDst[:])
	}

	binary.BigEndian.PutUint16(res[32:34], p.dport)
	binary.BigEndian.PutUint16(res[34:36], p.sport)
	res[36] = p.proto

	return
}

func (p testParams) genDummyPacket(pktType capture.PacketType) capture.Packet {
	EPHash, IsIPv4 := p.genEPHash()
	data := make([]byte, len(capturetypes.EPHash{})+ipv6.HeaderLen)

	if IsIPv4 {
		data[0] = (4 << 4)
		data[9] = p.proto
		copy(data[12:16], EPHash[0:4])
		copy(data[16:20], EPHash[16:20])
		copy(data[ipv4.HeaderLen:ipv4.HeaderLen+2], EPHash[34:36])
		copy(data[ipv4.HeaderLen+2:ipv4.HeaderLen+4], EPHash[32:34])

	} else {
		data[0] = (6 << 4)
		data[6] = p.proto
		copy(data[8:24], EPHash[0:16])
		copy(data[24:40], EPHash[16:32])
		copy(data[ipv6.HeaderLen:ipv6.HeaderLen+2], EPHash[34:36])
		copy(data[ipv6.HeaderLen+2:ipv6.HeaderLen+4], EPHash[32:34])
	}

	return capture.NewIPPacket(nil, data, pktType, 128, 0)
}

func uint16ToPort(p uint16) (res []byte) {
	res = make([]byte, 2)
	binary.BigEndian.PutUint16(res, p)
	return
}
