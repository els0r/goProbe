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

	"github.com/fako1024/gopacket"
	"github.com/fako1024/gopacket/layers"
)

var (
	byteArray1Zeros  = byte(0x00)
	byteArray2Zeros  = [2]byte{0x00, 0x00}
	byteArray4Zeros  = [4]byte{0x00, 0x00, 0x00, 0x00}
	byteArray16Zeros = [16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
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
	// core fields
	sip      [16]byte
	dip      [16]byte
	sport    [2]byte
	dport    [2]byte
	protocol byte
	numBytes uint16

	// direction indicator fields
	tcpFlags byte

	// packet descriptors
	epHash        EPHash
	epHashReverse EPHash
	dirInbound    bool // packet inbound or outbound on interface
}

func (p *GPPacket) computeEPHash() {
	// carve out the ports
	dport := uint16(p.dport[0])<<8 | uint16(p.dport[1])
	sport := uint16(p.sport[0])<<8 | uint16(p.sport[1])

	// prepare byte arrays:
	// include different fields into the hashing arrays in order to
	// discern between session based traffic and udp traffic. When
	// session based traffic is observed, the source port is taken
	// into account. A major exception is traffic over port 53 as
	// considering every single DNS request/response would
	// significantly fill up the flow map
	copy(p.epHash[0:], p.sip[:])
	copy(p.epHash[16:], p.dip[:])
	copy(p.epHash[32:], p.dport[:])
	if p.protocol == 6 && dport != 53 && sport != 53 {
		copy(p.epHash[34:], p.sport[:])
	} else {
		p.epHash[34], p.epHash[35] = 0, 0
	}
	p.epHash[36] = p.protocol

	copy(p.epHashReverse[0:], p.dip[:])
	copy(p.epHashReverse[16:], p.sip[:])
	copy(p.epHashReverse[32:], p.sport[:])
	if p.protocol == 6 && dport != 53 && sport != 53 {
		copy(p.epHashReverse[34:], p.dport[:])
	} else {
		p.epHashReverse[34], p.epHashReverse[35] = 0, 0
	}
	p.epHashReverse[36] = p.protocol
}

// Populate takes a raw packet and populates a GPPacket structure from it.
func (p *GPPacket) Populate(srcPacket gopacket.Packet) error {

	// first things first: reset packet from previous run
	p.reset()

	// size helper vars
	var nwLHeaderSize, tpHeaderSize uint16

	// process metadata
	p.numBytes = uint16(srcPacket.Metadata().CaptureInfo.Length)

	// read the direction from which the packet entered the interface
	p.dirInbound = false
	if srcPacket.Metadata().CaptureInfo.Inbound == 1 {
		p.dirInbound = true
	}

	// for ESP traffic (which lacks a transport layer)
	var skipTransport bool

	// decode packet
	if nwL := srcPacket.NetworkLayer(); nwL != nil {
		nwLC := nwL.LayerContents()
		nwLHeaderSize = uint16(len(nwLC))

		// exit if layer is available but the bytes aren't captured by the layer
		// contents
		if nwLHeaderSize == 0 {
			return fmt.Errorf("Network layer header not available")
		}

		// get ip info
		ipsrc, ipdst := nwL.NetworkFlow().Endpoints()
		copy(p.sip[:], ipsrc.Raw())
		copy(p.dip[:], ipdst.Raw())

		// read out the next layer protocol
		// the default value is reserved by IANA and thus will never occur unless
		// the protocol could not be correctly identified
		p.protocol = 0xFF
		switch nwL.LayerType() {
		case layers.LayerTypeIPv4:

			p.protocol = nwLC[9]

			// only run the fragmentation checks on fragmented TCP/UDP packets. For
			// ESP, we don't have any transport layer information so there's no
			// need to distinguish between ESP fragments or other ESP traffic
			//
			// Note: an ESP fragment will carry fragmentation information like any
			// other IP packet. The fragment offset will of be MTU - 20 bytes (IP layer).
			if p.protocol == ESP {
				skipTransport = true
			} else {
				// check for IP fragmentation
				fragBits := (0xe0 & nwLC[6]) >> 5
				fragOffset := (uint16(0x1f&nwLC[6]) << 8) | uint16(nwLC[7])

				// return decoding error if the packet carries anything other than the
				// first fragment, i.e. if the packet lacks a transport layer header
				if fragOffset != 0 {
					return fmt.Errorf("Fragmented IP packet: offset: %d flags: %d", fragOffset, fragBits)
				}
			}
		case layers.LayerTypeIPv6:
			p.protocol = nwLC[6]
		}

		if !skipTransport {
			if tpL := srcPacket.TransportLayer(); tpL != nil {

				// get layer contents
				tpLC := tpL.LayerContents()
				tpHeaderSize = uint16(len(tpLC))

				if tpHeaderSize == 0 {
					return fmt.Errorf("Transport layer header not available")
				}

				// get port bytes
				psrc, dsrc := tpL.TransportFlow().Endpoints()

				// only get raw bytes if we actually have TCP or UDP
				if p.protocol == TCP || p.protocol == UDP {
					copy(p.sport[:], psrc.Raw())
					copy(p.dport[:], dsrc.Raw())
				}

				// if the protocol is TCP, grab the flag information
				if p.protocol == TCP {
					if tpHeaderSize < 14 {
						return fmt.Errorf("Incomplete TCP header: %d", tpLC)
					}

					p.tcpFlags = tpLC[13] // we are primarily interested in SYN, ACK and FIN
				}
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

	p.computeEPHash()
	return nil
}

func (p *GPPacket) reset() {
	p.sip = byteArray16Zeros
	p.dip = byteArray16Zeros
	p.dport = byteArray2Zeros
	p.sport = byteArray2Zeros
	p.protocol = byteArray1Zeros
	p.numBytes = uint16(0)
	p.tcpFlags = byteArray1Zeros
	p.epHash = byteArray37Zeros
	p.epHashReverse = byteArray37Zeros
	p.dirInbound = false
}
