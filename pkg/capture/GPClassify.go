/////////////////////////////////////////////////////////////////////////////////
//
// GPClassify.go
//
// Wrapper file for the classifier function used to determine the direction of a
// packet with respect to its Endpoints
//
// Written by Lennart Elsen lel@open.ch, June 2014
// Copyright (c) 2014 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package capture

const (
	Unknown          uint8 = 0
	DirectionRemains uint8 = 1
	DirectionReverts uint8 = 2
)

/*
const multicastRangeIPv4 [3]byte = [3]byte{244, 0, 0}
const broadcastIPv4      [4]byte = [4]byte{255, 255, 255, 255}
const ssdpAddressIPv4    [4]byte = [4]byte{239, 255, 255, 250}
*/

// slice storing frequently used destination ports which fall outside of the service
// port range 1-1023. These are explicit exceptions to the direction heuristic below
var specialPorts [6]uint16 = [6]uint16{
	5222,  // XMPP, iMessage
	5353,  // DNS
	8080,  // Proxy
	8612,  // Canon BJNP
	17500, // Dropbox LanSync
	1352,  // Lotus Notes
}

func IsSpecialPort(port uint16) bool {
	special := false

	// check if port matches any of the special ports
	for _, p := range specialPorts {
		special = special || (p == port)
	}

	return special
}

// This function is responsible for running a variety of heuristics on the packet
// in order to determine its direction. This classification is important since the
// termination of flows in regular intervals otherwise results in the incapability
// to correctly assign the appropriate endpoints. Current heuristics include:
//   - investigating the TCP flags (if available)
//   - incorporating the port information (with respect to privileged ports)
//   - dissecting ICMP traffic
//
// Return value: according to above enumeration
//    0: if no classification possible
//    1: if packet direction is "request"
//    2: if packet direction is "response"
func ClassifyPacketDirection(packet *GPPacket) uint8 {
	sport := uint16(packet.sport[0])<<8 | uint16(packet.sport[1])
	dport := uint16(packet.dport[0])<<8 | uint16(packet.dport[1])

	// first, check the TCP flags availability
	TCPflags := packet.tcpFlags
	if TCPflags != 0x00 {
		// process the TCP handshake flags to decide the direction.
		switch TCPflags {
		case 0x02: // SYN
			return DirectionRemains
		case 0x12: // SYN-ACK
			return DirectionReverts
		case 0x01: // FIN
			return DirectionRemains
		case 0x11: // FIN-ACK
			return DirectionReverts
		}
	}

	// handle multicast addresses
	if (packet.dip[0] == 224 && packet.dip[1] == 0) && (packet.dip[2] == 0 || packet.dip[2] == 1) {
		return DirectionRemains
	}

	// handle broadcast address
	if packet.dip[0] == 255 && packet.dip[1] == 255 && packet.dip[2] == 255 && packet.dip[3] == 255 {
		return DirectionRemains
	}

	// handle TCP and UDP
	if packet.protocol == 6 || packet.protocol == 17 {
		// non-TCP-handshake packet encountered, but port information available

		// check for DHCP messages
		if dport == 67 && sport == 68 {
			return DirectionRemains
		}

		if dport == 68 && sport == 67 {
			return DirectionReverts
		}

		// according to RFC 6056, look for ephemeral ports in the range of 49152
		// through 65535
		if (dport < 1024 || IsSpecialPort(dport)) && sport > 20000 {
			return DirectionRemains
		}

		if (sport < 1024 || IsSpecialPort(sport)) && dport > 20000 {
			return DirectionReverts
		}
	}

	/* disregarded until further notice
	   if packet.ICMPtypeCode != 0xffff {
	       // try ICMP codes if protocol field matches. Echo request and reply can shed
	       // light on the situation. Other codes may help too
	       switch packet.ICMPtypeCode {
	       case 0x0000: // echo reply
	           return DirectionRemains
	       case 0x0800: // echo request
	           return DirectionReverts
	       }

	   }
	*/

	// if there is yet no verdict, return "Unknown"
	return Unknown
}
