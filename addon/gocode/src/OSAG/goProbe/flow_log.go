/////////////////////////////////////////////////////////////////////////////////
//
// flow_log.go
//
// Defines FlowLog for storing flows.
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, December 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goProbe

import "OSAG/goDB"

// A FlowLog stores flows. It is NOT threadsafe.
type FlowLog struct {
	// TODO(lob): Consider making this map[EPHash]GPFlow to reduce GC load
	flowMap map[EPHash]*GPFlow
}

// NewFlowLog creates a new flow log for storing flows.
func NewFlowLog() *FlowLog {
	return &FlowLog{make(map[EPHash]*GPFlow)}
}

// Add a packet to the flow log. If the packet belongs to a flow
// already present in the log, the flow will be updated. Otherwise,
// a new flow will be created.
func (fm *FlowLog) Add(packet *GPPacket) {
	// update or assign the flow
	if flowToUpdate, existsHash := fm.flowMap[packet.epHash]; existsHash {
		flowToUpdate.UpdateFlow(packet)
	} else if flowToUpdate, existsReverseHash := fm.flowMap[packet.epHashReverse]; existsReverseHash {
		flowToUpdate.UpdateFlow(packet)
	} else {
		fm.flowMap[packet.epHash] = NewGPFlow(packet)
	}
}

// Rotate the log. All flows are reset to no packets and traffic.
// Moreover, any flows not worth keeping (according to GPFlow.IsWorthKeeping)
// are discarded.
//
// Returns an AggFlowMap containing all flows since the last call to Rotate.
func (fm *FlowLog) Rotate() (agg goDB.AggFlowMap) {
	if len(fm.flowMap) == 0 {
		SysLog.Debug("There are currently no flow records available")
	}

	fm.flowMap, agg = fm.transferAndAggregate()

	return
}

func (fm *FlowLog) transferAndAggregate() (newFlowMap map[EPHash]*GPFlow, agg goDB.AggFlowMap) {
	newFlowMap = make(map[EPHash]*GPFlow)
	agg = make(goDB.AggFlowMap)

	for k, v := range fm.flowMap {

		// check if the flow actually has any interesting information for us
		if !v.HasBeenIdle() {
			var (
				tsip, tdip [16]byte
			)

			copy(tsip[:], v.sip[:])
			copy(tdip[:], v.dip[:])

			var tempkey = goDB.Key{
				tsip,
				tdip,
				[2]byte{v.dport[0], v.dport[1]},
				v.protocol,
			}

			if toUpdate, exists := agg[tempkey]; exists {
				toUpdate.NBytesRcvd += v.nBytesRcvd
				toUpdate.NBytesSent += v.nBytesSent
				toUpdate.NPktsRcvd += v.nPktsRcvd
				toUpdate.NPktsSent += v.nPktsSent
			} else {
				agg[tempkey] = &goDB.Val{v.nBytesRcvd, v.nBytesSent, v.nPktsRcvd, v.nPktsSent}
			}

			// check whether the flow should be retained for the next interval
			// or thrown away
			if v.IsWorthKeeping() {
				// reset and insert the flow into the new flow matrix
				v.Reset()
				newFlowMap[k] = v
			}
		}
	}

	return
}
