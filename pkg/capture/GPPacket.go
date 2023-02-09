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

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
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
func (p *GPPacket) Populate(srcPacket gopacket.Packet, inbound bool) error {

	// first things first: reset packet from previous run
	p.reset()

	// size helper vars
	var nlHeaderSize, tpHeaderSize uint16

	// process metadata
	p.numBytes = uint16(srcPacket.Metadata().CaptureInfo.Length)

	// read the direction from which the packet entered the interface
	p.dirInbound = inbound

	// for ESP traffic (which lacks a transport layer)
	var skipTransport bool

	// decode packet
	if srcPacket.NetworkLayer() != nil {
		nwL := srcPacket.NetworkLayer().LayerContents()
		nlHeaderSize = uint16(len(nwL))

		// exit if layer is available but the bytes aren't captured by the layer
		// contents
		if nlHeaderSize == 0 {
			return fmt.Errorf("Network layer header not available")
		}

		// get ip info
		ipsrc, ipdst := srcPacket.NetworkLayer().NetworkFlow().Endpoints()
		copy(p.epHash[0:16], ipsrc.Raw())
		copy(p.epHash[16:32], ipdst.Raw())
		copy(p.epHashReverse[0:16], p.epHash[16:32])
		copy(p.epHashReverse[16:32], p.epHash[0:16])

		// read out the next layer protocol
		// the default value is reserved by IANA and thus will never occur unless
		// the protocol could not be correctly identified
		var protocol byte = 0xFF
		switch srcPacket.NetworkLayer().LayerType() {
		case layers.LayerTypeIPv4:

			protocol = nwL[9]
			p.isIPv4 = true

			// only run the fragmentation checks on fragmented TCP/UDP packets. For
			// ESP, we don't have any transport layer information so there's no
			// need to distinguish between ESP fragments or other ESP traffic
			//
			// Note: an ESP fragment will carry fragmentation information like any
			// other IP packet. The fragment offset will of be MTU - 20 bytes (IP layer).
			if protocol == ESP {
				skipTransport = true
			} else {
				// check for IP fragmentation
				fragBits := (0xe0 & nwL[6]) >> 5
				fragOffset := (uint16(0x1f&nwL[6]) << 8) | uint16(nwL[7])

				// return decoding error if the packet carries anything other than the
				// first fragment, i.e. if the packet lacks a transport layer header
				if fragOffset != 0 {
					return fmt.Errorf("Fragmented IP packet: offset: %d flags: %d", fragOffset, fragBits)
				}
			}
		case layers.LayerTypeIPv6:
			protocol = nwL[6]
		}
		p.epHash[36], p.epHashReverse[36] = protocol, protocol

		if !skipTransport && srcPacket.TransportLayer() != nil {
			// get layer contents
			tpL := srcPacket.TransportLayer().LayerContents()
			tpHeaderSize = uint16(len(tpL))

			if tpHeaderSize == 0 {
				return fmt.Errorf("Transport layer header not available")
			}

			// get port bytes
			psrc, dsrc := srcPacket.TransportLayer().TransportFlow().Endpoints()

			// only get raw bytes if we actually have TCP or UDP
			if protocol == TCP || protocol == UDP {
				dport := dsrc.Raw()
				sport := psrc.Raw()
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

			// if the protocol is TCP, grab the flag information
			if protocol == TCP {
				if tpHeaderSize < 14 {
					return fmt.Errorf("Incomplete TCP header: %d", tpL)
				}

				p.tcpFlags = tpL[13] // we are primarily interested in SYN, ACK and FIN
			}
		}
	} else {

		// extract error if available
		if err := srcPacket.ErrorLayer(); err != nil {

			// enrich it with concrete info about which layer failed
			var layers string
			for _, layer := range srcPacket.Layers() {
				layers += layer.LayerType().String() + "/"
			}
			layers = layers[:len(layers)-1]
			return fmt.Errorf("%s: %s", layers, err.Error())
		}

		// if the error layer is nil, the packet belongs to a protocol which does not contain
		// IP layers and hence no useful information for goquery
		return nil
	}

	return nil
}

func (p *GPPacket) reset() {
	p.numBytes = uint16(0)
	p.tcpFlags = 0
	p.epHash = byteArray37Zeros
	p.epHashReverse = byteArray37Zeros
	p.dirInbound = false
}
