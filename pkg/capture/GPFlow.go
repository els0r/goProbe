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
	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
)

// GPFlow stores a goProbe flow
type GPFlow struct {
	epHash EPHash

	// Hash Map Value variables
	bytesRcvd       uint64
	bytesSent       uint64
	packetsRcvd     uint64
	packetsSent     uint64
	pktDirectionSet bool
	isIPv4          bool
}

// Key returns a goDB compliant key from the current flow
func (f *GPFlow) Key() (key types.Key) {
	if f.isIPv4 {
		key = types.NewV4Key(f.epHash[0:4], f.epHash[16:20], f.epHash[32:34], f.epHash[36])
	} else {
		key = types.NewV6Key(f.epHash[0:16], f.epHash[16:32], f.epHash[32:34], f.epHash[36])
	}
	return
}

// MarshalJSON implements the Marshaler interface for a flow
func (f *GPFlow) MarshalJSON() ([]byte, error) {
	return jsoniter.Marshal(
		struct {
			Hash EPHash `json:"hash"`

			// Hash Map Value variables
			BytesRcvd   uint64 `json:"bytesRcvd"`
			BytesSent   uint64 `json:"bytesSent"`
			PacketsRcvd uint64 `json:"packetsRcvd"`
			PacketsSent uint64 `json:"packetsSent"`
		}{
			f.epHash,
			f.bytesRcvd, f.bytesSent, f.packetsRcvd, f.packetsSent},
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
		epHash:          packet.epHash,
		pktDirectionSet: updateDirection(packet), // try to get the packet direction
		isIPv4:          packet.isIPv4,
	}

	// set packet and byte counters with respect to its interface direction
	if packet.dirInbound {
		res.bytesRcvd = uint64(packet.numBytes)
		res.packetsRcvd = 1
	} else {
		res.bytesSent = uint64(packet.numBytes)
		res.packetsSent = 1
	}

	return &res
}

// UpdateFlow increments flow counters if the packet belongs to an existing flow
func (f *GPFlow) UpdateFlow(packet *GPPacket) {

	// increment packet and byte counters with respect to its interface direction
	if packet.dirInbound {
		f.bytesRcvd += uint64(packet.numBytes)
		f.packetsRcvd++
	} else {
		f.bytesSent += uint64(packet.numBytes)
		f.packetsSent++
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
	return f.hasIdentifiedDirection() && !f.HasBeenIdle()
}

// Reset resets all flow counters
func (f *GPFlow) Reset() {
	f.bytesRcvd = 0
	f.bytesSent = 0
	f.packetsRcvd = 0
	f.packetsSent = 0
}

func (f *GPFlow) hasIdentifiedDirection() bool {
	return f.pktDirectionSet
}

// HasBeenIdle checks whether the flow has received packets into any direction. In the flow
// lifecycle this is the last stage.
//
//	New -> Update -> Reset -> Idle -> Delete
func (f *GPFlow) HasBeenIdle() bool {
	return (f.packetsRcvd == 0) && (f.packetsSent == 0)
}
