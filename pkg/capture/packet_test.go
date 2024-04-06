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
	input testParams

	isIPv4   bool
	EPHashV4 capturetypes.EPHashV4
	EPHashV6 capturetypes.EPHashV6
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

func TestEphemeralPort(t *testing.T) {
	require.Equal(t, uint16(32768), capturetypes.MinEphemeralPort, "Minimum ephemeral port is != []byte{128, 0}, adapt isEphemeralPort() accordingly !")
	require.Equal(t, uint16(65535), capturetypes.MaxEphemeralPort, "Maximum ephemeral port is != max(uint16), adapt isEphemeralPort() accordingly !")
}

func TestPortMergeLogic(t *testing.T) {
	for i := uint16(0); i < 65535; i++ {
		if i == 53 || i == 80 || i == 443 || i == 445 || i == 8080 {
			require.Truef(t, isCommonPort(uint16ToPort(i), capturetypes.TCP), "Port %d/TCP (%v) considered common port, adapt isCommonPort() accordingly !", i, uint16ToPort(i))
		} else {
			require.Falsef(t, isCommonPort(uint16ToPort(i), capturetypes.TCP), "Port %d/TCP (%v) not considered common port, adapt isCommonPort() accordingly !", i, uint16ToPort(i))
		}
		if i == 53 || i == 443 {
			require.Truef(t, isCommonPort(uint16ToPort(i), capturetypes.UDP), "Port %d/UDP (%v) considered common port, adapt isCommonPort() accordingly !", i, uint16ToPort(i))
		} else {
			require.Falsef(t, isCommonPort(uint16ToPort(i), capturetypes.UDP), "Port %d/UDP (%v) not considered common port, adapt isCommonPort() accordingly !", i, uint16ToPort(i))
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
			epHash, _, errno := ParsePacketV4(croppedIPLayer)
			require.Equal(t, capturetypes.ErrnoOK, errno, "population error")
			require.Equal(t, capturetypes.EPHashV4(refHash), epHash)
		} else {
			croppedIPLayer = testPacket.IPLayer()[:ipv6.HeaderLen]
			epHash, _, errno := ParsePacketV6(croppedIPLayer)
			require.Equal(t, capturetypes.ErrnoOK, errno, "population error")
			require.Equal(t, capturetypes.EPHashV6(refHash), epHash)
		}
	}
}

func TestPopulation(t *testing.T) {
	for _, params := range testCases {
		t.Run(params.String(), func(t *testing.T) {
			testPacket := params.genDummyPacket(0)
			refHash, refIsIPv4 := params.genEPHash()

			if refIsIPv4 {
				epHash, _, errno := ParsePacketV4(testPacket.IPLayer())
				require.Equal(t, capturetypes.ErrnoOK, errno, "population error")
				require.Equal(t, capturetypes.EPHashV4(refHash), epHash)
			} else {
				epHash, _, errno := ParsePacketV6(testPacket.IPLayer())
				require.Equal(t, capturetypes.ErrnoOK, errno, "population error")
				require.Equal(t, capturetypes.EPHashV6(refHash), epHash)
			}
		})
	}
}

func TestClassification(t *testing.T) {
	for _, params := range testCases {
		t.Run(params.String(), func(t *testing.T) {
			testCase := params.genTestCase()
			if testCase.isIPv4 {
				require.Equal(t, params.expectedDirection, capturetypes.ClassifyPacketDirectionV4(testCase.EPHashV4, testCase.input.AuxInfo), "classification mismatch")
			} else {
				require.Equal(t, params.expectedDirection, capturetypes.ClassifyPacketDirectionV6(testCase.EPHashV6, testCase.input.AuxInfo), "classification mismatch")
			}
		})
	}
}

var commonPortBenchResult bool // required to prevent compiler from optimizing below calls

func BenchmarkCommonPort(b *testing.B) {
	port := make([]byte, 2)

	b.Run("hit", func(b *testing.B) {
		binary.BigEndian.PutUint16(port, 10000)
		for i := 0; i < b.N; i++ {
			commonPortBenchResult = isCommonPort(port, capturetypes.TCP)
		}
	})
	b.Run("miss", func(b *testing.B) {
		binary.BigEndian.PutUint16(port, 443)
		for i := 0; i < b.N; i++ {
			commonPortBenchResult = isCommonPort(port, capturetypes.UDP)
		}
	})
}

func BenchmarkPopulation(b *testing.B) {
	for _, params := range testCases {
		testPacket := params.genDummyPacket(0)

		if testPacket.IPLayer().Type() == ipLayerTypeV4 {
			b.Run(params.String(), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _, _ = ParsePacketV4(testPacket.IPLayer())
				}
			})
		} else {
			b.Run(params.String(), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _, _ = ParsePacketV6(testPacket.IPLayer())
				}
			})
		}
	}
}

func BenchmarkClassification(b *testing.B) {
	for _, params := range testCases {
		testCase := params.genTestCase()

		if testCase.isIPv4 {
			b.Run(params.String(), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					capturetypes.ClassifyPacketDirectionV4(testCase.EPHashV4, testCase.input.AuxInfo)
				}
			})
		} else {
			b.Run(params.String(), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					capturetypes.ClassifyPacketDirectionV6(testCase.EPHashV6, testCase.input.AuxInfo)
				}
			})
		}
	}
}

func (p testParams) genTestCase() testCase {
	epHashData, IsIPv4 := p.genEPHash()

	res := testCase{
		input:  p,
		isIPv4: IsIPv4,
	}

	if IsIPv4 {
		res.EPHashV4 = capturetypes.EPHashV4(epHashData)
	} else {
		res.EPHashV6 = capturetypes.EPHashV6(epHashData)
	}

	return res
}

func (p testParams) genEPHash() (res []byte, IsIPv4 bool) {

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
		epHash := capturetypes.EPHashV4{}

		tmpSrc, tmpDst := src.As4(), dst.As4()
		copy(epHash[capturetypes.EPHashV4SipStart:], tmpSrc[:])
		copy(epHash[capturetypes.EPHashV4DipStart:], tmpDst[:])
		binary.BigEndian.PutUint16(epHash[capturetypes.EPHashV4DPortStart:capturetypes.EPHashV4DPortEnd], p.dport)
		binary.BigEndian.PutUint16(epHash[capturetypes.EPHashV4SPortStart:capturetypes.EPHashV4SPortEnd], p.sport)
		epHash[capturetypes.EPHashV4ProtocolPos] = p.proto

		res = epHash[:]
	} else {
		epHash := capturetypes.EPHashV6{}

		tmpSrc, tmpDst := src.As16(), dst.As16()
		copy(epHash[capturetypes.EPHashV6SipStart:], tmpSrc[:])
		copy(epHash[capturetypes.EPHashV6DipStart:], tmpDst[:])
		binary.BigEndian.PutUint16(epHash[capturetypes.EPHashV6DPortStart:capturetypes.EPHashV6DPortEnd], p.dport)
		binary.BigEndian.PutUint16(epHash[capturetypes.EPHashV6SPortStart:capturetypes.EPHashV6SPortEnd], p.sport)
		epHash[capturetypes.EPHashV6ProtocolPos] = p.proto

		res = epHash[:]
	}

	return
}

func (p testParams) genDummyPacket(pktType capture.PacketType) capture.Packet {
	epHashData, IsIPv4 := p.genEPHash()
	data := make([]byte, len(capturetypes.EPHashV6{})+ipv6.HeaderLen)

	if IsIPv4 {
		epHash := capturetypes.EPHashV4(epHashData)

		data[0] = (4 << 4)
		data[ipLayerV4ProtoPos] = p.proto
		copy(data[ipLayerV4SipStart:ipLayerV4SipEnd], epHash[capturetypes.EPHashV4SipStart:capturetypes.EPHashV4SipEnd])
		copy(data[ipLayerV4DipStart:ipLayerV4DipEnd], epHash[capturetypes.EPHashV4DipStart:capturetypes.EPHashV4DipEnd])
		copy(data[ipLayerV4SPortStart:ipLayerV4SPortEnd], epHash[capturetypes.EPHashV4SPortStart:capturetypes.EPHashV4SPortEnd])
		copy(data[ipLayerV4DPortStart:ipLayerV4DPortEnd], epHash[capturetypes.EPHashV4DPortStart:capturetypes.EPHashV4DPortEnd])

	} else {
		epHash := capturetypes.EPHashV6(epHashData)

		data[0] = (6 << 4)
		data[ipLayerV6ProtoPos] = p.proto
		copy(data[ipLayerV6SipStart:ipLayerV6SipEnd], epHash[capturetypes.EPHashV6SipStart:capturetypes.EPHashV6SipEnd])
		copy(data[ipLayerV6DipStart:ipLayerV6DipEnd], epHash[capturetypes.EPHashV6DipStart:capturetypes.EPHashV6DipEnd])
		copy(data[ipLayerV6SPortStart:ipLayerV6SPortEnd], epHash[capturetypes.EPHashV6SPortStart:capturetypes.EPHashV6SPortEnd])
		copy(data[ipLayerV6DPortStart:ipLayerV6DPortEnd], epHash[capturetypes.EPHashV6DPortStart:capturetypes.EPHashV6DPortEnd])
	}

	return capture.NewIPPacket(nil, data, pktType, 128, 0)
}

func uint16ToPort(p uint16) (res []byte) {
	res = make([]byte, 2)
	binary.BigEndian.PutUint16(res, p)
	return
}
