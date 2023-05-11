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

// populate takes a raw packet and populates a GPPacket structure from it.
func populate(p *capturetypes.GPPacket, pkt capture.Packet) error {
	// Extract the IP layer of the packet
	srcPacket := pkt.IPLayer()

	// Ascertain the direction from which the packet entered the interface
	p.DirInbound = pkt.IsInbound()
	p.NumBytes = pkt.TotalLen()
	var protocol byte

	if ipLayerType := srcPacket.Type(); ipLayerType == ipLayerTypeV4 {

		p.IsIPv4 = true

		// Parse IPv4 packet information
		copy(p.EPHash[0:4], srcPacket[12:16])
		copy(p.EPHash[16:20], srcPacket[16:20])
		copy(p.EPHashReverse[0:4], p.EPHash[16:20])
		copy(p.EPHashReverse[16:20], p.EPHash[0:4])

		protocol = srcPacket[9]

		// only run the fragmentation checks on fragmented TCP/UDP packets. For
		// ESP, we don't have any transport layer information so there's no
		// need to distinguish between ESP fragments or other ESP traffic
		//
		// Note: an ESP fragment will carry fragmentation information like any
		// other IP packet. The fragment offset will of be MTU - 20 bytes (IP layer).
		if protocol != capturetypes.ESP {

			// check for IP fragmentation
			fragBits := (0xe0 & srcPacket[6]) >> 5
			fragOffset := (uint16(0x1f&srcPacket[6]) << 8) | uint16(srcPacket[7])

			// return decoding error if the packet carries anything other than the
			// first fragment, i.e. if the packet lacks a transport layer header
			if fragOffset != 0 {
				return fmt.Errorf("Fragmented IP packet: offset: %d flags: %d", fragOffset, fragBits)
			}
		}

		if protocol == capturetypes.TCP || protocol == capturetypes.UDP {

			dport := srcPacket[ipv4.HeaderLen+2 : ipv4.HeaderLen+4]
			sport := srcPacket[ipv4.HeaderLen : ipv4.HeaderLen+2]

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
				p.AuxInfo = srcPacket[ipv4.HeaderLen+13] // store TCP flags
			}
		} else if protocol == capturetypes.ICMP {
			p.AuxInfo = srcPacket[ipv4.HeaderLen] // store ICMP type
		}

	} else if ipLayerType == ipLayerTypeV6 {

		p.IsIPv4 = false

		// Parse IPv6 packet information
		copy(p.EPHash[0:16], srcPacket[8:24])
		copy(p.EPHash[16:32], srcPacket[24:40])
		copy(p.EPHashReverse[0:16], p.EPHash[16:32])
		copy(p.EPHashReverse[16:32], p.EPHash[0:16])

		protocol = srcPacket[6]
		if protocol == capturetypes.TCP || protocol == capturetypes.UDP {

			dport := srcPacket[ipv6.HeaderLen+2 : ipv6.HeaderLen+4]
			sport := srcPacket[ipv6.HeaderLen : ipv6.HeaderLen+2]

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
				p.AuxInfo = srcPacket[ipv6.HeaderLen+13] // store TCP flags
			}
		} else if protocol == capturetypes.ICMPv6 {
			p.AuxInfo = srcPacket[ipv6.HeaderLen] // store ICMP type
		}

	} else {
		return fmt.Errorf("received neither IPv4 nor IPv6 IP header: %v", srcPacket)
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
