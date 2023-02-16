/////////////////////////////////////////////////////////////////////////////////
//
// GPPacket.go
//
// Main packet Interface that provides the datastructure that is passed around
// every channel within the program. Contains the necessary information that a flow
// needs
//
// Written by Lennart Elsen lel@open.ch, May 2014
// Copyright (c) 2014 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package capture

import (
	"fmt"

	"github.com/fako1024/slimcap/capture"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

var (
	byteArray37Zeros = [37]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
)

// Enumeration of the most common IP protocols
const (
	TCP byte = 6
	UDP      = 17
	ESP      = 50
)

// EPHash is a typedef that allows us to replace the type of hash
type EPHash [37]byte

// GPPacket stores all relevant packet details for a flow
type GPPacket struct {

	// packet size
	numBytes uint16

	// direction indicator fields
	tcpFlags byte

	// packet inbound or outbound on interface
	dirInbound bool

	// flag to easily determine if a packet is IPv4 or IPv6
	isIPv4 bool

	// packet descriptors / hashes
	epHash        EPHash
	epHashReverse EPHash
}

// Populate takes a raw packet and populates a GPPacket structure from it.
func (p *GPPacket) Populate(pkt capture.Packet) error {

	// first things first: reset packet from previous run
	p.reset()
	srcPacket := pkt.IPLayer()

	// read the direction from which the packet entered the interface
	p.dirInbound = pkt.Type() == 0
	p.numBytes = uint16(pkt.TotalLen())
	var protocol byte = 0xFF

	if int(srcPacket[0]>>4) == 4 {

		p.isIPv4 = true

		// Parse IPv4 packet information
		copy(p.epHash[0:4], srcPacket[12:16])
		copy(p.epHash[16:20], srcPacket[16:20])
		copy(p.epHashReverse[0:4], p.epHash[16:20])
		copy(p.epHashReverse[16:20], p.epHash[0:4])

		protocol = srcPacket[9]

		// only run the fragmentation checks on fragmented TCP/UDP packets. For
		// ESP, we don't have any transport layer information so there's no
		// need to distinguish between ESP fragments or other ESP traffic
		//
		// Note: an ESP fragment will carry fragmentation information like any
		// other IP packet. The fragment offset will of be MTU - 20 bytes (IP layer).
		if protocol != ESP {

			// check for IP fragmentation
			fragBits := (0xe0 & srcPacket[6]) >> 5
			fragOffset := (uint16(0x1f&srcPacket[6]) << 8) | uint16(srcPacket[7])

			// return decoding error if the packet carries anything other than the
			// first fragment, i.e. if the packet lacks a transport layer header
			if fragOffset != 0 {
				return fmt.Errorf("Fragmented IP packet: offset: %d flags: %d", fragOffset, fragBits)
			}
		}

		if protocol == 6 || protocol == 17 {

			dport := srcPacket[ipv4.HeaderLen+2 : ipv4.HeaderLen+4]
			sport := srcPacket[ipv4.HeaderLen : ipv4.HeaderLen+2]

			copy(p.epHash[32:34], dport)
			copy(p.epHashReverse[32:34], sport)

			// If session based traffic is observed, the source port is taken
			// into account. A major exception is traffic over port 53 as
			// considering every single DNS request/response would
			// significantly fill up the flow map
			if protocol == 6 && (dport[0] != 0 || dport[1] != 53) && (sport[0] != 0 || sport[1] != 53) {
				copy(p.epHash[34:36], sport)
				copy(p.epHashReverse[34:36], dport)
			}
		}
	} else if int(srcPacket[0]>>4) == 6 {

		p.isIPv4 = false

		// Parse IPv6 packet information
		copy(p.epHash[0:16], srcPacket[8:24])
		copy(p.epHash[16:32], srcPacket[24:40])
		copy(p.epHashReverse[0:16], p.epHash[16:32])
		copy(p.epHashReverse[16:32], p.epHash[0:16])

		protocol = srcPacket[6]

		if protocol == 6 || protocol == 17 {

			dport := srcPacket[ipv6.HeaderLen+2 : ipv6.HeaderLen+4]
			sport := srcPacket[ipv6.HeaderLen : ipv6.HeaderLen+2]

			copy(p.epHash[32:34], dport)
			copy(p.epHashReverse[32:34], sport)

			// If session based traffic is observed, the source port is taken
			// into account. A major exception is traffic over port 53 as
			// considering every single DNS request/response would
			// significantly fill up the flow map
			if protocol == TCP && (dport[0] != 0 || dport[1] != 53) && (sport[0] != 0 || sport[1] != 53) {
				copy(p.epHash[34:36], sport)
				copy(p.epHashReverse[34:36], dport)
			}

			if protocol == TCP {
				p.tcpFlags = srcPacket[ipv6.HeaderLen+13]
			}
		}
	} else {
		return fmt.Errorf("received neither IPv4 nor IPv6 IP header: %v", srcPacket)
	}

	p.epHash[36], p.epHashReverse[36] = protocol, protocol

	return nil
}

func (p *GPPacket) reset() {
	p.numBytes = uint16(0)
	p.tcpFlags = 0
	p.epHash = byteArray37Zeros
	p.epHashReverse = byteArray37Zeros
	p.dirInbound = false
}
