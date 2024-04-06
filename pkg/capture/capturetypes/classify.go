package capturetypes

// ClassifyPacketDirectionV4 is responsible for running a variety of heuristics on IPv4 packets
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
func ClassifyPacketDirectionV4(epHash EPHashV4, auxInfo byte) Direction {

	// Check IP protocol
	switch epHash[EPHashV4ProtocolPos] {
	case TCP:

		// Use the TCP handshake to determine the direction
		if auxInfo != 0x00 {

			// Handshake stage
			if auxInfo&tcpFlagSYN != 0 {
				if auxInfo&tcpFlagACK != 0 {

					// SYN-ACK
					return DirectionReverts
				}

				// SYN
				return DirectionRemains
			}
		}

		return classifyByPortsV4(epHash)

	case UDP:

		// Handle broadcast / multicast addresses (we do not need to check the
		// inverse direction because it won't be in multicast format)
		if isBroadcastMulticastV4(epHash[EPHashV4DipStart:EPHashV4DipEnd]) {
			return DirectionRemains
		}

		return classifyByPortsV4(epHash)

	case ICMP:
		return classifyICMPv4(auxInfo)

	default:
	}

	// if there is no verdict, return "Unknown"
	return DirectionUnknown
}

// ClassifyPacketDirectionV6 is responsible for running a variety of heuristics on IPv6 packets
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
func ClassifyPacketDirectionV6(epHash EPHashV6, auxInfo byte) Direction {

	// Check IP protocol
	switch epHash[EPHashV6ProtocolPos] {
	case TCP:

		// Use the TCP handshake to determine the direction
		if auxInfo != 0x00 {

			// Handshake stage
			if auxInfo&tcpFlagSYN != 0 {
				if auxInfo&tcpFlagACK != 0 {

					// SYN-ACK
					return DirectionReverts
				}

				// SYN
				return DirectionRemains
			}
		}

		return classifyByPortsV6(epHash)

	case UDP:

		// Handle broadcast / multicast addresses (we do not need to check the
		// inverse direction because it won't be in multicast format)
		if isBroadcastMulticastV6(epHash[EPHashV6DipStart:EPHashV6DipEnd]) {
			return DirectionRemains
		}

		return classifyByPortsV6(epHash)

	case ICMPv6:
		return classifyICMPv6(epHash, auxInfo)

	default:
	}

	// if there is no verdict, return "Unknown"
	return DirectionUnknown
}

const (
	tcpFlagSYN = 0x02
	tcpFlagACK = 0x10
)

const (
	icmpV4EchoReply              = 0x00
	icmpV4DestinationUnreachable = 0x03
	icmpV4EchoRrequest           = 0x08
	icmpV4TimeExceeded           = 0x0B
	icmpV4ParameterProblem       = 0x0C
	icmpV4TimestampRequest       = 0x0D
	icmpV4TimestampReply         = 0x0E
)

func classifyICMPv4(icmpType byte) Direction {

	// Check the ICMPv4 Type parameter
	switch icmpType {

	// Reply-type ICMP v4 messages
	case icmpV4EchoReply, icmpV4DestinationUnreachable, icmpV4TimeExceeded, icmpV4ParameterProblem, icmpV4TimestampReply:
		return DirectionReverts

	// Request-type ICMP v4 messages
	case icmpV4EchoRrequest, icmpV4TimestampRequest:
		return DirectionRemains
	}

	return DirectionUnknown
}

const (
	icmpV6EchoReply              = 0x81
	icmpV6DestinationUnreachable = 0x01
	icmpV6TimeExceeded           = 0x03
	icmpV6ParameterProblem       = 0x04
	icmpV6EchoRrequest           = 0x80
)

func classifyICMPv6(epHash EPHashV6, icmpType byte) Direction {

	// Handle broadcast / multicast addresses (we do not need to check the
	// inverse direction because it won't be in multicast format)
	if isBroadcastMulticastV6(epHash[EPHashV6DipStart:EPHashV6DipEnd]) {
		return DirectionRemains
	}

	// Check the ICMPv6 Type parameter
	switch icmpType {

	// Reply-type ICMP v6 messages
	case icmpV6EchoReply, icmpV6DestinationUnreachable, icmpV6TimeExceeded, icmpV6ParameterProblem:
		return DirectionReverts

	// Request-type ICMP v6 messages
	case icmpV6EchoRrequest:
		return DirectionRemains
	}

	return DirectionUnknown
}

func classifyByPortsV4(epHash EPHashV4) Direction {

	// Compiler hint
	_ = epHash[EPHashV4ProtocolPos]

	// Source port is ephemeral
	if isEphemeralPort(epHash[EPHashV4SPortStart:EPHashV4SPortEnd]) {

		// Destination port is not ephemeral -> Probably this is client -> server
		if !isEphemeralPort(epHash[EPHashV4DPortStart:EPHashV4DPortEnd]) {
			return DirectionRemains
		}

		// Destination port is ephemeral as well
		// If destination port is smaller than the source port -> Probably this is client -> server
		if epHash[EPHashV4DPortFirstByte] < epHash[EPHashV4SPortFirstByte] ||
			(epHash[EPHashV4DPortFirstByte] == epHash[EPHashV4SPortFirstByte] && epHash[EPHashV4DPortLastByte] < epHash[EPHashV4SPortLastByte]) {
			return DirectionRemains

			// If source port is smaller than the destination port -> Probably this is server -> client
		} else if epHash[EPHashV4SPortFirstByte] < epHash[EPHashV4DPortFirstByte] ||
			(epHash[EPHashV4SPortFirstByte] == epHash[EPHashV4DPortFirstByte] && epHash[EPHashV4SPortLastByte] < epHash[EPHashV4DPortLastByte]) {
			return DirectionReverts
		}

		// Source port is not ephemeral
	} else {

		// Destination port is ephemeral -> Probably this is server -> client
		if isEphemeralPort(epHash[EPHashV4DPortStart:EPHashV4DPortEnd]) {
			return DirectionReverts
		}

		// Destination port is not ephemeral either
		// If source port is smaller than the destination port -> Probably this is server -> client
		if epHash[EPHashV4SPortFirstByte] < epHash[EPHashV4DPortFirstByte] ||
			(epHash[EPHashV4SPortFirstByte] == epHash[EPHashV4DPortFirstByte] && epHash[EPHashV4SPortLastByte] < epHash[EPHashV4DPortLastByte]) {
			return DirectionReverts

			// If destination port is smaller than the source  port -> Probably this is client -> server
		} else if epHash[EPHashV4DPortFirstByte] < epHash[EPHashV4SPortFirstByte] ||
			(epHash[EPHashV4DPortFirstByte] == epHash[EPHashV4SPortFirstByte] && epHash[EPHashV4DPortLastByte] < epHash[EPHashV4SPortLastByte]) {
			return DirectionRemains
		}
	}

	// Ports are identical, we have nothing to go by and can only assume this is the first packet
	return DirectionRemains
}

func classifyByPortsV6(epHash EPHashV6) Direction {

	// Compiler hint
	_ = epHash[EPHashV6ProtocolPos]

	// Source port is ephemeral
	if isEphemeralPort(epHash[EPHashV6SPortStart:EPHashV6SPortEnd]) {

		// Destination port is not ephemeral -> Probably this is client -> server
		if !isEphemeralPort(epHash[EPHashV6DPortStart:EPHashV6DPortEnd]) {
			return DirectionRemains
		}

		// Destination port is ephemeral as well
		// If destination port is smaller than the source port -> Probably this is client -> server
		if epHash[EPHashV6DPortFirstByte] < epHash[EPHashV6SPortFirstByte] ||
			(epHash[EPHashV6DPortFirstByte] == epHash[EPHashV6SPortFirstByte] && epHash[EPHashV6DPortLastByte] < epHash[EPHashV6SPortLastByte]) {
			return DirectionRemains

			// If source port is smaller than the destination port -> Probably this is server -> client
		} else if epHash[EPHashV6SPortFirstByte] < epHash[EPHashV6DPortFirstByte] ||
			(epHash[EPHashV6SPortFirstByte] == epHash[EPHashV6DPortFirstByte] && epHash[EPHashV6SPortLastByte] < epHash[EPHashV6DPortLastByte]) {
			return DirectionReverts
		}

		// Source port is not ephemeral
	} else {

		// Destination port is ephemeral -> Probably this is server -> client
		if isEphemeralPort(epHash[EPHashV6DPortStart:EPHashV6DPortEnd]) {
			return DirectionReverts
		}

		// Destination port is not ephemeral either
		// If source port is smaller than the destination port -> Probably this is server -> client
		if epHash[EPHashV6SPortFirstByte] < epHash[EPHashV6DPortFirstByte] ||
			(epHash[EPHashV6SPortFirstByte] == epHash[EPHashV6DPortFirstByte] && epHash[EPHashV6SPortLastByte] < epHash[EPHashV6DPortLastByte]) {
			return DirectionReverts

			// If destination port is smaller than the source  port -> Probably this is client -> server
		} else if epHash[EPHashV6DPortFirstByte] < epHash[EPHashV6SPortFirstByte] ||
			(epHash[EPHashV6DPortFirstByte] == epHash[EPHashV6SPortFirstByte] && epHash[EPHashV6DPortLastByte] < epHash[EPHashV6SPortLastByte]) {
			return DirectionRemains
		}
	}

	// Ports are identical, we have nothing to go by and can only assume this is the first packet
	return DirectionRemains
}

// Ephemeral ports as union of:
// -> suggested by IANA / RFC6335 (49152–65535)
// -> used by most Linux kernels (32768–60999)
const (
	MaxEphemeralPort uint16 = 65535
	MinEphemeralPort uint16 = 32768 // matches condition in isEphemeralPort()
)

func isEphemeralPort(port []byte) bool {

	// Since maxEphemeralPort is 65535 we don't need to check the upper bound
	// Matches MinEphemeralPort (32768 -> []byte{128, 0})
	return port[0] >= 128 ||

		// We consider an empty port to be ephemaral (because it indicates that the source port was disregarded)
		(port[0] == 0 && port[1] == 0)
}

func isBroadcastMulticastV4(destinationIP []byte) bool {

	// These comparisons are more clumsy than using e.g. bytes.Equal, but they are faster
	return (destinationIP[0] == 0xFF && destinationIP[1] == 0xFF && destinationIP[2] == 0xFF && destinationIP[3] == 0xFF) ||
		((destinationIP[0] == 0xE0 && destinationIP[1] == 0x00) && (destinationIP[2] == 0x00 || destinationIP[2] == 0x01))
}

func isBroadcastMulticastV6(destinationIP []byte) bool {

	// IPv6 only has the concept of multicast addresses (there are no "broadcasts")
	// According to RFC4291:
	// IPv6 multicast addresses are distinguished from unicast addresses by the
	// value of the high-order octet of the addresses: a value of 0xFF (binary
	// 11111111) identifies an address as a multicast address; any other value
	// identifies an address as a unicast address.
	return destinationIP[0] == 0xFF
}
