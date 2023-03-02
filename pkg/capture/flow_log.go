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

package capture

import (
	"fmt"
	"text/tabwriter"

	"github.com/els0r/goProbe/pkg/goDB/protocols"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/els0r/log"
	jsoniter "github.com/json-iterator/go"
)

// constants for table printing
const (
	headerStrUpper = "\t\t\t\t\t\t\tbytes\tbytes\tpackets\tpackets\t"
	headerStr      = "\tsip\tsport\t\tdip\tdport\tproto\trcvd\tsent\trcvd\tsent\t"
	fmtStr         = "%s\t%s\t%d\t←―→\t%s\t%d\t%s\t%d\t%d\t%d\t%d\t\n"
)

// FlowLog stores flows. It is NOT threadsafe.
type FlowLog struct {
	flowMap map[string]*GPFlow
	logger  log.Logger
}

// NewFlowLog creates a new flow log for storing flows.
func NewFlowLog(logger log.Logger) *FlowLog {
	return &FlowLog{make(map[string]*GPFlow), logger}
}

// MarshalJSON implements the jsoniter.Marshaler interface
func (f *FlowLog) MarshalJSON() ([]byte, error) {
	var toMarshal []interface{}
	for _, v := range f.flowMap {
		toMarshal = append(toMarshal, v)
	}
	return jsoniter.Marshal(toMarshal)
}

// Len returns the number of flows in the FlowLog
func (f *FlowLog) Len() int {
	return len(f.flowMap)
}

// Flows provides an iterator for the internal flow map
func (f *FlowLog) Flows() map[string]*GPFlow {
	return f.flowMap
}

// TablePrint pretty prints the flows in a formatted table
func (f *FlowLog) TablePrint(w *tabwriter.Writer) error {
	fmt.Fprintln(w, headerStrUpper)
	fmt.Fprintln(w, headerStr)
	for _, g := range f.Flows() {
		prefix := "["
		var state string
		if g.HasBeenIdle() {
			state += "!"
		}
		if g.pktDirectionSet {
			state += "*"
		}
		if state == "" {
			prefix = ""
		} else {
			prefix += state + "]"
		}

		fmt.Fprintf(w, fmtStr,
			prefix,
			types.RawIPToString(g.epHash[0:16]),
			types.PortToUint16(g.epHash[34:36]),
			types.RawIPToString(g.epHash[16:32]),
			types.PortToUint16(g.epHash[32:34]),
			protocols.GetIPProto(int(g.epHash[36])),
			g.bytesRcvd, g.bytesSent, g.packetsRcvd, g.packetsSent)
	}
	return w.Flush()
}

// Add a packet to the flow log. If the packet belongs to a flow
// already present in the log, the flow will be updated. Otherwise,
// a new flow will be created.
func (f *FlowLog) Add(packet *GPPacket) {
	// update or assign the flow
	if flowToUpdate, existsHash := f.flowMap[string(packet.epHash[:])]; existsHash {
		flowToUpdate.UpdateFlow(packet)
	} else if flowToUpdate, existsReverseHash := f.flowMap[string(packet.epHashReverse[:])]; existsReverseHash {
		flowToUpdate.UpdateFlow(packet)
	} else {
		f.flowMap[string(packet.epHash[:])] = NewGPFlow(packet)
	}
}

// Rotate rotates the flow log. All flows are reset to no packets and traffic.
// Moreover, any flows not worth keeping (according to GPFlow.IsWorthKeeping)
// are discarded.
//
// Returns an AggFlowMap containing all flows since the last call to Rotate.
func (f *FlowLog) Rotate() (agg *hashmap.AggFlowMap) {
	if len(f.flowMap) == 0 {
		f.logger.Debug("There are currently no flow records available")
	}

	f.flowMap, agg = f.transferAndAggregate()

	return
}

func (f *FlowLog) transferAndAggregate() (newFlowMap map[string]*GPFlow, agg *hashmap.AggFlowMap) {
	newFlowMap = make(map[string]*GPFlow)
	agg = hashmap.NewAggFlowMap()

	for k, v := range f.flowMap {

		goDBKey := v.Key()

		// check if the flow actually has any interesting information for us
		if !v.HasBeenIdle() {
			if v.isIPv4 {
				agg.V4Map.SetOrUpdate(goDBKey, v.bytesRcvd, v.bytesSent, v.packetsRcvd, v.packetsSent)
			} else {
				agg.V6Map.SetOrUpdate(goDBKey, v.bytesRcvd, v.bytesSent, v.packetsRcvd, v.packetsSent)
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
