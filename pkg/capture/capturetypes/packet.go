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

// EPHashV4 is a typedef that allows us to replace the type of hash for IPv4 flows
// epHash[0:4] -> Src IP
// epHash[4:6] -> Src Port
// epHash[6:10] -> Dst IP
// epHash[10:12] -> Dst Port
// epHash[12] -> Protocol
type EPHashV4 [EPHashSizeV4]byte

// EPHashV6 is a typedef that allows us to replace the type of hash for IPv6 flows
// epHash[0:16] -> Src IP
// epHash[16:18] -> Src Port
// epHash[18:34] -> Dst IP
// epHash[34:36] -> Dst Port
// epHash[36] -> Protocol
type EPHashV6 [EPHashSizeV6]byte

// Reverse calculates the reverse of an EPHashV4 (i.e. source / destination switched)
func (h EPHashV4) Reverse() (rev EPHashV4) {
	copy(rev[0:6], h[6:12])
	copy(rev[6:12], h[0:6])
	rev[12] = h[12]

	return
}

// IsProbablyReverse performs a very simple heuristic in order to determine if a packet
// is most likely to be classified as forward or backward (hence allowing to optimize
// the flow map lookup path)
func (h EPHashV4) IsProbablyReverse() bool {

	// If the (alleged) source port is zero, we either removed it or this isn't even
	// a TCP / UDP packet, so we proceed normally
	if h[4] == 0 && h[5] == 0 {
		return false
	}

	// If the (alleged) destination port is zero (bur the source port wasn't) this was a common
	// port and we nulled it, so we assume it was reverted (because the source port was a common one)
	if h[10] == 0 && h[11] == 0 {
		return true
	}

	// If the most significant byte is already smaller for the (alleged) source port this
	// is probably a reverse packet
	if h[4] < h[10] {
		return true
	}

	// If the most significant bytes are equal, check again for the least significant one and
	// follow the same logic
	if h[4] == h[10] {
		return h[5] < h[11]
	}

	// Nothing of the above, so we proceed normally
	return false
}

// Reverse calculates the reverse of an EPHashV6 (i.e. source / destination switched)
func (h EPHashV6) Reverse() (rev EPHashV6) {
	copy(rev[0:18], h[18:36])
	copy(rev[18:36], h[0:18])
	rev[36] = h[36]

	return
}

// IsProbablyReverse performs a very simple heuristic in order to determine if a packet
// is most likely to be classified as forward or backward (hence allowing to optimize
// the flow map lookup path)
func (h EPHashV6) IsProbablyReverse() bool {

	// If the (alleged) source port is zero, we either removed it or this isn't even
	// a TCP / UDP packet, so we proceed normally
	if h[16] == 0 && h[17] == 0 {
		return false
	}

	// If the (alleged) destination port is zero (bur the source port wasn't) this was a common
	// port and we nulled it, so we assume it was reverted (because the source port was a common one)
	if h[34] == 0 && h[35] == 0 {
		return true
	}

	// If the most significant byte is already smaller for the (alleged) source port this
	// is probably a reverse packet
	if h[16] < h[34] {
		return true
	}

	// If the most significant bytes are equal, check again for the least significant one and
	// follow the same logic
	if h[16] == h[34] {
		return h[17] < h[35]
	}

	// Nothing of the above, so we proceed normally
	return false
}
