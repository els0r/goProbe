package capturetypes

// Direction denotes if the detected packet direction should remain or changed, based
// on flow analysis
type Direction uint8

// Direction detection states
const (
	DirectionUnknown Direction = iota
	DirectionRemains
	DirectionReverts
)

// Enumeration of the most common IP protocols
const (
	ICMP   = 0x01 // ICMP : 1
	TCP    = 0x06 // TCP :  6
	UDP    = 0x11 // UDP : 17
	ESP    = 0x32 // ESP : 50
	ICMPv6 = 0x3A // ICMPv6 : 58

	EPHashSizeV4 = 13 // EPHashSizeV4 : The (static) length of an IPv4 EPHash
	EPHashSizeV6 = 37 // EPHashSizeV6 : The (static) length of an IPv6 EPHash
)

// EPHashV4 array position constants (all explicit so they can theoretically be switched
// around with zero effort and to avoid having to do index math in functions)
// epHash[0:4] -> Src IP
// epHash[4:6] -> Src Port
// epHash[6:10] -> Dst IP
// epHash[10:12] -> Dst Port
// epHash[12] -> Protocol
const (
	EPHashV4SipStart    = 0
	EPHashV4SipEnd      = 4
	EPHashV4SPortStart  = 4
	EPHashV4SPortEnd    = 6
	EPHashV4DipStart    = 6
	EPHashV4DipEnd      = 10
	EPHashV4DPortStart  = 10
	EPHashV4DPortEnd    = 12
	EPHashV4ProtocolPos = 12

	EPHashV4SPortFirstByte = EPHashV4SPortStart     // 4
	EPHashV4SPortLastByte  = EPHashV4SPortStart + 1 // 5
	EPHashV4DPortFirstByte = EPHashV4DPortStart     // 10
	EPHashV4DPortLastByte  = EPHashV4DPortStart + 1 // 11
)

// EPHashV4 is a typedef that allows us to replace the type of hash for IPv4 flows
type EPHashV4 [EPHashSizeV4]byte

// EPHashV6 array position constants (all explicit so they can theoretically be switched
// around with zero effort and to avoid having to do index math in functions)
// epHash[0:16] -> Src IP
// epHash[16:18] -> Src Port
// epHash[18:34] -> Dst IP
// epHash[34:36] -> Dst Port
// epHash[36] -> Protocol
const (
	EPHashV6SipStart    = 0
	EPHashV6SipEnd      = 16
	EPHashV6SPortStart  = 16
	EPHashV6SPortEnd    = 18
	EPHashV6DipStart    = 18
	EPHashV6DipEnd      = 34
	EPHashV6DPortStart  = 34
	EPHashV6DPortEnd    = 36
	EPHashV6ProtocolPos = 36

	EPHashV6SPortFirstByte = EPHashV6SPortStart     // 16
	EPHashV6SPortLastByte  = EPHashV6SPortStart + 1 // 17
	EPHashV6DPortFirstByte = EPHashV6DPortStart     // 34
	EPHashV6DPortLastByte  = EPHashV6DPortStart + 1 // 35
)

// EPHashV6 is a typedef that allows us to replace the type of hash for IPv6 flows
type EPHashV6 [EPHashSizeV6]byte

// Reverse calculates the reverse of an EPHashV4 (i.e. source / destination switched)
func (h EPHashV4) Reverse() (rev EPHashV4) {

	// Switch source / destination IP & port information
	copy(rev[EPHashV4SipStart:EPHashV4SPortEnd], h[EPHashV4DipStart:EPHashV4DPortEnd])
	copy(rev[EPHashV4DipStart:EPHashV4DPortEnd], h[EPHashV4SipStart:EPHashV4SPortEnd])

	// Copy protocol information as is
	rev[EPHashV4ProtocolPos] = h[EPHashV4ProtocolPos]

	return
}

// IsProbablyReverse performs a very simple heuristic in order to determine if a packet
// is most likely to be classified as forward or backward (hence allowing to optimize
// the flow map lookup path)
func (h EPHashV4) IsProbablyReverse() bool {

	// If the (alleged) source port is zero, we either removed it or this isn't even
	// a TCP / UDP packet, so we proceed normally
	if h[EPHashV4SPortFirstByte] == 0 && h[EPHashV4SPortLastByte] == 0 {
		return false
	}

	// If the (alleged) destination port is zero (bur the source port wasn't) this was a common
	// port and we nulled it, so we assume it was reverted (because the source port was a common one)
	if h[EPHashV4DPortFirstByte] == 0 && h[EPHashV4DPortLastByte] == 0 {
		return true
	}

	// If the most significant byte is already smaller for the (alleged) source port this
	// is probably a reverse packet
	if h[EPHashV4SPortFirstByte] < h[EPHashV4DPortFirstByte] {
		return true
	}

	// If the most significant bytes are equal, check again for the least significant one and
	// follow the same logic
	if h[EPHashV4SPortFirstByte] == h[EPHashV4DPortFirstByte] {
		return h[EPHashV4SPortLastByte] < h[EPHashV4DPortLastByte]
	}

	// Nothing of the above, so we proceed normally
	return false
}

// Reverse calculates the reverse of an EPHashV6 (i.e. source / destination switched)
func (h EPHashV6) Reverse() (rev EPHashV6) {

	// Switch source / destination IP & port information
	copy(rev[EPHashV6SipStart:EPHashV6SPortEnd], h[EPHashV6DipStart:EPHashV6DPortEnd])
	copy(rev[EPHashV6DipStart:EPHashV6DPortEnd], h[EPHashV6SipStart:EPHashV6SPortEnd])

	// Copy protocol information as is
	rev[EPHashV6ProtocolPos] = h[EPHashV6ProtocolPos]

	return
}

// IsProbablyReverse performs a very simple heuristic in order to determine if a packet
// is most likely to be classified as forward or backward (hence allowing to optimize
// the flow map lookup path)
func (h EPHashV6) IsProbablyReverse() bool {

	// If the (alleged) source port is zero, we either removed it or this isn't even
	// a TCP / UDP packet, so we proceed normally
	if h[EPHashV6SPortFirstByte] == 0 && h[EPHashV6SPortLastByte] == 0 {
		return false
	}

	// If the (alleged) destination port is zero (bur the source port wasn't) this was a common
	// port and we nulled it, so we assume it was reverted (because the source port was a common one)
	if h[EPHashV6DPortFirstByte] == 0 && h[EPHashV6DPortLastByte] == 0 {
		return true
	}

	// If the most significant byte is already smaller for the (alleged) source port this
	// is probably a reverse packet
	if h[EPHashV6SPortFirstByte] < h[EPHashV6DPortFirstByte] {
		return true
	}

	// If the most significant bytes are equal, check again for the least significant one and
	// follow the same logic
	if h[EPHashV6SPortFirstByte] == h[EPHashV6DPortFirstByte] {
		return h[EPHashV6SPortLastByte] < h[EPHashV6DPortLastByte]
	}

	// Nothing of the above, so we proceed normally
	return false
}
