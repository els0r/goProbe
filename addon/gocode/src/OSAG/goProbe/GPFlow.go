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
    l7proto         uint16
    nBytesRcvd      uint64
    nBytesSent      uint64
    nPktsRcvd       uint64
    nPktsSent       uint64
    pktDirectionSet bool

    // store the layer 7 payload coming from a return packet
    pktPayloadOtherDirection    [4]byte
    pktPayloadLenOtherDirection uint32

    canDPI bool
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
        layer7proto                                  uint16
        payloadOtherDir                              [4]byte
        payloadLenOtherDir                           uint32
        canDPI                                       bool
    )

    // set packet and byte counters with respect to its interface direction
    if packet.dirInbound {
        bytes_rcvd = uint64(packet.numBytes)
        pkts_rcvd = 1
    } else {
        bytes_sent = uint64(packet.numBytes)
        pkts_sent = 1
    }

    // assign current packet payload to the other direction
    payloadOtherDir = packet.l7payload
    payloadLenOtherDir = uint32(packet.l7payloadSize)

    sport := uint16(packet.sport[0])<<8 | uint16(packet.sport[1])
    dport := uint16(packet.dport[0])<<8 | uint16(packet.dport[1])

    // try to get the packet direction
    directionSet := updateDirection(packet)

    if packet.protocol == TCP || packet.protocol == UDP {
        canDPI = true
    }

    // try to get the layer 7 protocol only if there's a transport
    // layer for the packet
    if canDPI {
        layer7proto = dpiPtr.GetLayer7Proto(packet.l7payload,
            [4]byte{0x00, 0x00, 0x00, 0x00},
            uint32(packet.l7payloadSize),
            uint32(0),
            dport,
            sport,
            packet.protocol,
            uint32(packet.l7payloadSize),
            uint32(0),
            packet.dip,
            packet.sip)
    }

    return &GPFlow{packet.sip, packet.dip, packet.sport, packet.dport, packet.protocol, layer7proto, bytes_rcvd, bytes_sent, pkts_rcvd, pkts_sent, directionSet, payloadOtherDir, payloadLenOtherDir, canDPI}
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

    sport := uint16(packet.sport[0])<<8 | uint16(packet.sport[1])
    dport := uint16(packet.dport[0])<<8 | uint16(packet.dport[1])

    // update layer 7 protocol in case it was not detected with the first packet
    if f.canDPI {
        if !(f.hasIdentifiedL7Proto()) {
            f.l7proto = dpiPtr.GetLayer7Proto(packet.l7payload,
                f.pktPayloadOtherDirection,
                uint32(packet.l7payloadSize),
                f.pktPayloadLenOtherDirection,
                dport,
                sport,
                packet.protocol,
                uint32(packet.l7payloadSize),
                f.pktPayloadLenOtherDirection,
                packet.dip,
                packet.sip)
        }
        // assign current packet payload to the other direction
        f.pktPayloadOtherDirection = packet.l7payload
        f.pktPayloadLenOtherDirection = uint32(packet.l7payloadSize)
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
    if f.hasIdentifiedL7Proto() || f.hasIdentifiedDirection() {

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

func (f *GPFlow) hasIdentifiedL7Proto() bool {
    return f.l7proto > 3
}

func (f *GPFlow) hasIdentifiedDirection() bool {
    return f.pktDirectionSet
}

func (f *GPFlow) HasBeenIdle() bool {
    return (f.nPktsRcvd == 0) && (f.nPktsSent == 0)
}
