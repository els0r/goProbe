package capture

import (
	"fmt"

	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/fako1024/slimcap/capture"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	ipLayerTypeV4 = 0x04 // 4
	ipLayerTypeV6 = 0x06 // 6
)

// Populate takes a raw packet and populates a GPPacket structure from it.
func Populate(p *capturetypes.GPPacket, ipLayer capture.IPLayer, pktType capture.PacketType, pktTotalLen uint32) error {

	// Ascertain the direction from which the packet entered the interface
	p.DirInbound = pktType != capture.PacketOutgoing
	p.NumBytes = pktTotalLen
	var protocol byte

	if ipLayerType := ipLayer.Type(); ipLayerType == ipLayerTypeV4 {

		p.IsIPv4 = true

		// Parse IPv4 packet information
		copy(p.EPHash[0:4], ipLayer[12:16])
		copy(p.EPHash[16:20], ipLayer[16:20])
		copy(p.EPHashReverse[0:4], p.EPHash[16:20])
		copy(p.EPHashReverse[16:20], p.EPHash[0:4])

		protocol = ipLayer[9]

		// only run the fragmentation checks on fragmented TCP/UDP packets. For
		// ESP, we don't have any transport layer information so there's no
		// need to distinguish between ESP fragments or other ESP traffic
		//
		// Note: an ESP fragment will carry fragmentation information like any
		// other IP packet. The fragment offset will of be MTU - 20 bytes (IP layer).
		if protocol != capturetypes.ESP {

			// check for IP fragmentation
			fragBits := (0xe0 & ipLayer[6]) >> 5
			fragOffset := (uint16(0x1f&ipLayer[6]) << 8) | uint16(ipLayer[7])

			// return decoding error if the packet carries anything other than the
			// first fragment, i.e. if the packet lacks a transport layer header
			if fragOffset != 0 {
				return fmt.Errorf("fragmented IP packet: offset: %d flags: %d", fragOffset, fragBits)
			}
		}

		if protocol == capturetypes.TCP || protocol == capturetypes.UDP {

			dport := ipLayer[ipv4.HeaderLen+2 : ipv4.HeaderLen+4]
			sport := ipLayer[ipv4.HeaderLen : ipv4.HeaderLen+2]

			// If session based traffic is observed, the source port is taken
			// into account. A major exception is traffic over port 53 as
			// considering every single DNS request/response would
			// significantly fill up the flow map
			if !isCommonPort(dport, protocol) {
				copy(p.EPHash[34:36], sport)
				copy(p.EPHashReverse[32:34], sport)
			}
			if !isCommonPort(sport, protocol) {
				copy(p.EPHash[32:34], dport)
				copy(p.EPHashReverse[34:36], dport)
			}

			if protocol == capturetypes.TCP {
				if len(ipLayer) < ipv4.HeaderLen+13 {
					return fmt.Errorf("tcp packet too short")
				}
				p.AuxInfo = ipLayer[ipv4.HeaderLen+13] // store TCP flags
			}
		} else if protocol == capturetypes.ICMP {
			p.AuxInfo = ipLayer[ipv4.HeaderLen] // store ICMP type
		}

	} else if ipLayerType == ipLayerTypeV6 {

		p.IsIPv4 = false

		// Parse IPv6 packet information
		copy(p.EPHash[0:16], ipLayer[8:24])
		copy(p.EPHash[16:32], ipLayer[24:40])
		copy(p.EPHashReverse[0:16], p.EPHash[16:32])
		copy(p.EPHashReverse[16:32], p.EPHash[0:16])

		protocol = ipLayer[6]
		if protocol == capturetypes.TCP || protocol == capturetypes.UDP {

			dport := ipLayer[ipv6.HeaderLen+2 : ipv6.HeaderLen+4]
			sport := ipLayer[ipv6.HeaderLen : ipv6.HeaderLen+2]

			// If session based traffic is observed, the source port is taken
			// into account. A major exception is traffic over port 53 as
			// considering every single DNS request/response would
			// significantly fill up the flow map
			if !isCommonPort(dport, protocol) {
				copy(p.EPHash[34:36], sport)
				copy(p.EPHashReverse[32:34], sport)
			}
			if !isCommonPort(sport, protocol) {
				copy(p.EPHash[32:34], dport)
				copy(p.EPHashReverse[34:36], dport)
			}

			if protocol == capturetypes.TCP {
				if len(ipLayer) < ipv6.HeaderLen+13 {
					return fmt.Errorf("tcp packet too short")
				}
				p.AuxInfo = ipLayer[ipv6.HeaderLen+13] // store TCP flags
			}
		} else if protocol == capturetypes.ICMPv6 {
			p.AuxInfo = ipLayer[ipv6.HeaderLen] // store ICMP type
		}

	} else {
		return fmt.Errorf("received neither IPv4 nor IPv6 IP header: %v", ipLayer)
	}

	p.EPHash[36], p.EPHashReverse[36] = protocol, protocol

	return nil
}

func isCommonPort(port []byte, proto byte) bool {
	// Fast path for neither of the below
	if port[0] > 1 {
		return false
	}

	// TCP common ports
	if proto == capturetypes.TCP {
		return (port[0] == 0 && (port[1] == 53 || port[1] == 80)) || // DNS(TCP), HTTP
			(port[0] == 1 && port[1] == 187) // HTTPS
	}

	// UDP common ports
	if proto == capturetypes.UDP {
		return (port[0] == 0 && port[1] == 53) || // DNS(UDP)
			(port[0] == 1 && port[1] == 187) // 443(UDP)
	}

	return false
}
