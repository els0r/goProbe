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

package capture

import (
	"github.com/els0r/goProbe/pkg/goDB"
	jsoniter "github.com/json-iterator/go"
)

// GPFlow stores a goProbe flow
type GPFlow struct {
	epHash EPHash

	// Hash Map Value variables
	nBytesRcvd      uint64
	nBytesSent      uint64
	nPktsRcvd       uint64
	nPktsSent       uint64
	pktDirectionSet bool
	isIPv4          bool
}

// Key returns a goDB compliant key from the current flow
func (f *GPFlow) Key() (key goDB.Key) {
	if f.isIPv4 {
		key = goDB.NewV4Key(f.epHash[0:4], f.epHash[16:20], f.epHash[32:34], f.epHash[36])
	} else {
		key = goDB.NewV6Key(f.epHash[0:16], f.epHash[16:32], f.epHash[32:34], f.epHash[36])
	}
	return
}

// MarshalJSON implements the Marshaler interface for a flow
func (f *GPFlow) MarshalJSON() ([]byte, error) {
	return jsoniter.Marshal(
		struct {
			Hash EPHash `json:"hash"`

			// Hash Map Value variables
			NBytesRcvd uint64 `json:"bytesRcvd"`
			NBytesSent uint64 `json:"bytesSent"`
			NPktsRcvd  uint64 `json:"packetsRcvd"`
			NPktsSent  uint64 `json:"packetsSent"`
		}{
			f.epHash,
			f.nBytesRcvd, f.nBytesSent, f.nPktsRcvd, f.nPktsSent},
	)
}

func updateDirection(packet *GPPacket) bool {
	directionSet := false
	if direction := ClassifyPacketDirection(packet); direction != Unknown {
		directionSet = true

		// switch fields if direction was opposite to the default direction
		// "DirectionRemains"
		if direction == DirectionReverts {
			packet.epHash, packet.epHashReverse = packet.epHashReverse, packet.epHash
		}
	}

	return directionSet
}

// NewGPFlow creates a new flow based on the packet
func NewGPFlow(packet *GPPacket) *GPFlow {
	res := GPFlow{
		epHash:          make(EPHash, len(packet.epHash)),
		pktDirectionSet: updateDirection(packet), // try to get the packet direction
		isIPv4:          packet.isIPv4,
	}

	// set packet and byte counters with respect to its interface direction
	if packet.dirInbound {
		res.nBytesRcvd = uint64(packet.numBytes)
		res.nPktsRcvd = 1
	} else {
		res.nBytesSent = uint64(packet.numBytes)
		res.nPktsSent = 1
	}

	// Populate hash (copy required to allow for re-use of packet)
	copy(res.epHash, packet.epHash)

	return &res
}

// UpdateFlow increments flow counters if the packet belongs to an existing flow
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

// IsWorthKeeping is used by a flow to check whether it has any interesting direction into
// worth keeping and whether its counters are non-zero. If they are, it means that
// the flow was essentially idle in the last time interval and that it can be safely
// discarded.
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

// Reset resets all flow counters
func (f *GPFlow) Reset() {
	f.nBytesRcvd = 0
	f.nBytesSent = 0
	f.nPktsRcvd = 0
	f.nPktsSent = 0
}

func (f *GPFlow) hasIdentifiedDirection() bool {
	return f.pktDirectionSet
}

// HasBeenIdle checks whether the flow has received packets into any direction. In the flow
// lifecycle this is the last stage.
//
//	New -> Update -> Reset -> Idle -> Delete
func (f *GPFlow) HasBeenIdle() bool {
	return (f.nPktsRcvd == 0) && (f.nPktsSent == 0)
}
