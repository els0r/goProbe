/////////////////////////////////////////////////////////////////////////////////
//
// GPFlow.go
//
// Main flow structure which is put into the GPMatrix and which is updated according to packet information
//
// Written by Lennart Elsen lel@open.ch, May 2014
// Copyright (c) 2014 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goProbe

type GPFlower interface {
	UpdateFlow()
	IsWorthKeeping() bool
	HasBeenIdle() bool
	Reset()
}

type GPFlow struct {
	// Hash Map Key variables
	sip      [16]byte
	dip      [16]byte
	sport    [2]byte
	dport    [2]byte
	protocol byte

	// Hash Map Value variables
	nBytesRcvd      uint64
	nBytesSent      uint64
	nPktsRcvd       uint64
	nPktsSent       uint64
	pktDirectionSet bool
}

func updateDirection(packet *GPPacket) bool {
	directionSet := false
	if direction := ClassifyPacketDirection(packet); direction != Unknown {
		directionSet = true

		// switch fields if direction was opposite to the default direction
		// "DirectionRemains"
		if direction == DirectionReverts {
			packet.sip, packet.dip = packet.dip, packet.sip
			packet.sport, packet.dport = packet.dport, packet.sport
		}
	}

	return directionSet
}

// Constructor method
func NewGPFlow(packet *GPPacket) *GPFlow {
	var (
		bytes_sent, bytes_rcvd, pkts_sent, pkts_rcvd uint64
	)

	// set packet and byte counters with respect to its interface direction
	if packet.dirInbound {
		bytes_rcvd = uint64(packet.numBytes)
		pkts_rcvd = 1
	} else {
		bytes_sent = uint64(packet.numBytes)
		pkts_sent = 1
	}

	// try to get the packet direction
	directionSet := updateDirection(packet)

	return &GPFlow{packet.sip, packet.dip, packet.sport, packet.dport, packet.protocol, bytes_rcvd, bytes_sent, pkts_rcvd, pkts_sent, directionSet}
}

// here, the values are incremented if the packet belongs to an existing flow
func (f *GPFlow) UpdateFlow(packet *GPPacket) {

	// increment packet and byte counters with respect to its interface direction
	if packet.dirInbound {
		f.nBytesRcvd += uint64(packet.numBytes)
		f.nPktsRcvd++
	} else {
		f.nBytesSent += uint64(packet.numBytes)
		f.nPktsSent++
	}

	// try to update direction if necessary
	if !(f.pktDirectionSet) {
		f.pktDirectionSet = updateDirection(packet)
	}
}

// routine that a flow uses to check whether it has any interesting layer 7 info
// worth keeping and whether its counters are non-zero. If they are, it means that
// the flow was essentially idle in the last time interval and that it can be safely
// discarded.
// Updated: also carries over the flows where a direction could be identified
func (f *GPFlow) IsWorthKeeping() bool {

	// first check if the flow stores and identified the layer 7 protocol or if the
	// flow stores direction information
	if f.hasIdentifiedDirection() {

		// check if any entries have been updated lately
		if !(f.HasBeenIdle()) {
			return true
		}
	}

	return false
}

// reset all flow counters
func (f *GPFlow) Reset() {
	f.nBytesRcvd = 0
	f.nBytesSent = 0
	f.nPktsRcvd = 0
	f.nPktsSent = 0
}

func (f *GPFlow) hasIdentifiedDirection() bool {
	return f.pktDirectionSet
}

func (f *GPFlow) HasBeenIdle() bool {
	return (f.nPktsRcvd == 0) && (f.nPktsSent == 0)
}
