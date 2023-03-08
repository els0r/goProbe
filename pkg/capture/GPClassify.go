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

type Direction uint8

// Direction detection states
const (
	Unknown Direction = iota
	DirectionRemains
	DirectionReverts
	DirectionMaybeRemains
	DirectionMaybeReverts
)

// IsConfidenceHigh returns if the heuristic was successful in determining the flow direction
// with high confidence (i.e. we trust the assessment and won't perform any further analysis)
func (d Direction) IsConfidenceHigh() bool {
	return d == DirectionRemains || d == DirectionReverts
}

// ClassifyPacketDirection is responsible for running a variety of heuristics on the packet
// in order to determine its direction. This classification is important since the
// termination of flows in regular intervals otherwise results in the incapability
// to correctly assign the appropriate endpoints. Current heuristics include:
//   - investigating the TCP flags (if available)
//   - incorporating the port information (with respect to privileged ports)
//   - dissecting ICMP traffic
//
// Return value: according to above enumeration
//
//	0: if no classification possible
//	1: if packet direction is "request" (with high confidence)
//	2: if packet direction is "response" (with high confidence)
//	3: if packet direction is "request" (with low confidence -> continue to assess)
//	4: if packet direction is "response" (with low confidence -> continue to assess)
func ClassifyPacketDirection(packet *GPPacket) Direction {

	// Check IP protocol
	switch packet.epHash[36] {
	case TCP:
		return classifyTCP(packet)

	case UDP:
		return classifyUDP(packet)

	case ICMP:
		return classifyICMPv4(packet)

	case ICMPv6:
		return classifyICMPv6(packet)

	default:
	}

	// if there is no verdict, return "Unknown"
	return Unknown
}

const (
	tcpFlagSYN = 0x02
	tcpFlagACK = 0x10
)

func classifyTCP(packet *GPPacket) Direction {

	// Use the TCP handshake to determine the direction
	if tcpFlags := packet.transportLayer[13]; tcpFlags != 0x00 {

		// Handshake stage
		if tcpFlags&tcpFlagSYN != 0 {
			if tcpFlags&tcpFlagACK != 0 {

				// SYN-ACK
				return DirectionReverts
			}

			// SYN
			return DirectionRemains
		}
	}

	return classifyByPorts(packet)
}

func classifyUDP(packet *GPPacket) Direction {

	// handle broadcast / multicast addresses (we do not need to check the
	// inverse direction because it won't be in multicast format)
	if isBroadcastMulticast(packet.epHash[16:32], packet.isIPv4) {
		return DirectionRemains
	}

	return classifyByPorts(packet)
}

func classifyICMPv4(packet *GPPacket) Direction {

	// Check the ICMPv4 Type parameter
	switch packet.transportLayer[0] {

	// EchoReply, DestinationUnreachable, TimeExceeded, ParameterProblem, TimestampReply
	case 0x00, 0x03, 0x0B, 0x0C, 0x0E:
		return DirectionReverts

	// EchoRrequest, TimestampRequest
	case 0x08, 0x0D:
		return DirectionRemains
	}

	return Unknown
}

func classifyICMPv6(packet *GPPacket) Direction {

	// handle broadcast / multicast addresses (we do not need to check the
	// inverse direction because it won't be in multicast format)
	if isBroadcastMulticast(packet.epHash[16:32], packet.isIPv4) {
		return DirectionRemains
	}

	// Check the ICMPv6 Type parameter
	switch packet.transportLayer[0] {

	// EchoReply, DestinationUnreachable, TimeExceeded, ParameterProblem
	case 0x81, 0x01, 0x03, 0x04:
		return DirectionReverts

	// EchoRequest
	case 0x80:
		return DirectionRemains
	}

	return Unknown
}

func classifyByPorts(packet *GPPacket) Direction {

	sport := uint16(packet.epHash[34])<<8 | uint16(packet.epHash[35])
	dport := uint16(packet.epHash[32])<<8 | uint16(packet.epHash[33])

	// Source port is ephemeral
	if isEphemeralPort(sport) {

		// Destination port is not ephemeral -> Probably this is client -> server
		if !isEphemeralPort(dport) {
			return DirectionRemains

			// Destination port is ephemeral as well
		} else {

			// If destination port is smaller than the source port -> Probably this is client -> server
			if dport < sport {
				return DirectionRemains

				// If source port is smaller than the destination port -> Probably this is server -> client
			} else if sport < dport {
				return DirectionReverts
			}
		}

		// Source port is not ephemeral
	} else {

		// Destination port is ephemeral -> Probably this is server -> client
		if isEphemeralPort(dport) {
			return DirectionReverts

			// Destination port is not ephemeral either
		} else {

			// If source port is smaller than the destination port -> Probably this is server -> client
			if sport < dport {
				return DirectionReverts

				// If destination port is smaller than the source  port -> Probably this is client -> server
			} else if dport < sport {
				return DirectionRemains
			}
		}
	}

	// Ports are identical, we have nothing to go by and can only assume this is the first packet
	return DirectionRemains
}

// Ephemeral ports as union of:
// -> suggested by IANA / RFC6335 (49152–65535)
// -> used by most Linux kernels (32768–60999)
const (
	minEphemeralPort uint16 = 32768
	maxEphemeralPort uint16 = 65535
)

func isEphemeralPort(port uint16) bool {
	return port >= minEphemeralPort || // Since maxEphemeralPort is 65535 we don't need to check the upper bound
		port == 0 // We consider an empty port to be ephemaral (because it indicates that the source port was disregarded)
}

func isBroadcastMulticast(destinationIP []byte, isIPv4 bool) bool {

	if isIPv4 {
		// These comparisons are more clumsy than using e.g. bytes.Equal, but they are faster
		return (destinationIP[0] == 0xFF && destinationIP[1] == 0xFF && destinationIP[2] == 0xFF && destinationIP[3] == 0xFF) ||
			((destinationIP[0] == 0xE0 && destinationIP[1] == 0x00) && (destinationIP[2] == 0x00 || destinationIP[2] == 0x01))
	}

	// IPv6 only has the concept of multicast addresses (there are no "broadcasts")
	// According to RFC4291:
	// IPv6 multicast addresses are distinguished from unicast addresses by the
	// value of the high-order octet of the addresses: a value of 0xFF (binary
	// 11111111) identifies an address as a multicast address; any other value
	// identifies an address as a unicast address.
	return destinationIP[0] == 0xFF
}
