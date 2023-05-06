package types

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
import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/els0r/goProbe/pkg/goDB/protocols"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	jsoniter "github.com/json-iterator/go"
)

// FlowLog stores flows. It is NOT threadsafe.
type FlowLog struct {
	flowMap map[string]*GPFlow
}

// NewFlowLog creates a new flow log for storing flows.
func NewFlowLog() *FlowLog {
	return &FlowLog{make(map[string]*GPFlow)}
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

// Add a packet to the flow log. If the packet belongs to a flow
// already present in the log, the flow will be updated. Otherwise,
// a new flow will be created.
func (f *FlowLog) Add(packet *GPPacket) {
	// update or assign the flow
	if flowToUpdate, existsHash := f.flowMap[string(packet.EPHash[:])]; existsHash {
		flowToUpdate.UpdateFlow(packet)
	} else if flowToUpdate, existsReverseHash := f.flowMap[string(packet.EPHashReverse[:])]; existsReverseHash {
		flowToUpdate.UpdateFlow(packet)
	} else {
		f.flowMap[string(packet.EPHash[:])] = NewGPFlow(packet)
	}
}

// Rotate rotates the flow log. All flows are reset to no packets and traffic.
// Moreover, any flows not worth keeping (according to GPFlow.IsWorthKeeping)
// are discarded.
//
// Returns an AggFlowMap containing all flows since the last call to Rotate.
func (f *FlowLog) Rotate() (agg *hashmap.AggFlowMap) {
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
			agg.SetOrUpdate(goDBKey, v.isIPv4, v.bytesRcvd, v.bytesSent, v.packetsRcvd, v.packetsSent)

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

// EPHash is a typedef that allows us to replace the type of hash
type EPHash [37]byte

// GPFlow stores a goProbe flow
type GPFlow struct {
	epHash EPHash

	// Hash Map Value variables
	bytesRcvd               uint64
	bytesSent               uint64
	packetsRcvd             uint64
	packetsSent             uint64
	directionConfidenceHigh bool
	isIPv4                  bool
}

func (f *GPFlow) DirectionConfidenceHigh() bool {
	return f.directionConfidenceHigh
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

func (f *GPFlow) PacketsRcvd() uint64 {
	return f.packetsRcvd
}

// MarshalJSON implements the Marshaler interface for a flow
func (f *GPFlow) MarshalJSON() ([]byte, error) {
	return jsoniter.Marshal(f.ToExtendedRow())
}

func (f *GPFlow) ToExtendedRow() results.ExtendedRow {
	return results.ExtendedRow{
		Attributes: results.ExtendedAttributes{
			SrcPort: types.PortToUint16(f.epHash[34:36]),
			Attributes: results.Attributes{
				SrcIP:   types.RawIPToAddr(f.epHash[0:16]),
				DstIP:   types.RawIPToAddr(f.epHash[16:32]),
				DstPort: types.PortToUint16(f.epHash[32:34]),
				IPProto: uint8(f.epHash[36]),
			},
		},
		Counters: types.Counters{
			BytesRcvd:   f.bytesRcvd,
			BytesSent:   f.bytesSent,
			PacketsRcvd: f.packetsRcvd,
			PacketsSent: f.packetsSent,
		},
	}
}

func (f *GPFlow) updateDirection(packet *GPPacket) {
	if direction := ClassifyPacketDirection(packet); direction != DirectionUnknown {
		f.directionConfidenceHigh = direction.IsConfidenceHigh()

		// switch fields if direction was opposite to the default direction
		// "DirectionRemains"
		if direction == DirectionReverts || direction == DirectionMaybeReverts {
			f.epHash = packet.EPHashReverse
		}
	}

	return
}

// NewGPFlow creates a new flow based on the packet
func NewGPFlow(packet *GPPacket) *GPFlow {

	res := GPFlow{
		epHash: packet.EPHash,
		isIPv4: packet.IsIPv4,
	}
	res.updateDirection(packet)

	// set packet and byte counters with respect to its interface direction
	if packet.DirInbound {
		res.bytesRcvd = uint64(packet.NumBytes)
		res.packetsRcvd = 1
	} else {
		res.bytesSent = uint64(packet.NumBytes)
		res.packetsSent = 1
	}

	return &res
}

// UpdateFlow increments flow counters if the packet belongs to an existing flow
func (f *GPFlow) UpdateFlow(packet *GPPacket) {

	// increment packet and byte counters with respect to its interface direction
	if packet.DirInbound {
		f.bytesRcvd += uint64(packet.NumBytes)
		f.packetsRcvd++
	} else {
		f.bytesSent += uint64(packet.NumBytes)
		f.packetsSent++
	}

	// try to update direction if necessary (as long as we're not confident enough)
	if !f.directionConfidenceHigh {
		f.updateDirection(packet)
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
	return f.directionConfidenceHigh
}

// HasBeenIdle checks whether the flow has received packets into any direction. In the flow
// lifecycle this is the last stage.
//
//	New -> Update -> Reset -> Idle -> Delete
func (f *GPFlow) HasBeenIdle() bool {
	return (f.packetsRcvd == 0) && (f.packetsSent == 0)
}

// FlowInfo summarizes information about a given flow
type FlowInfo struct {
	Idle                    bool                `json:"idle"`
	DirectionConfidenceHigh bool                `json:"direction_confidence_high"`
	Flow                    results.ExtendedRow `json:"flow"`
}

// FlowInfos is a list of FlowInfo objects
type FlowInfos []FlowInfo

// constants for table printing
const (
	headerStrUpper = "\t\t\t\t\t\t\tbytes\tbytes\tpackets\tpackets\t"
	headerStr      = "\tsip\tsport\t\tdip\tdport\tproto\trcvd\tsent\trcvd\tsent\t"
	fmtStr         = "%s\t%s\t%d\t←―→\t%s\t%d\t%s\t%d\t%d\t%d\t%d\t\n"
)

// TablePrint prints the list of flow infos in a formatted table
func (fs FlowInfos) TablePrint(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', tabwriter.AlignRight)

	fmt.Fprintln(tw, headerStrUpper)
	fmt.Fprintln(tw, headerStr)

	for _, fi := range fs {
		prefix := "["
		var state string
		if fi.Idle {
			state += "!"
		}
		if fi.DirectionConfidenceHigh {
			state += "*"
		}
		if state == "" {
			prefix = ""
		} else {
			prefix += state + "]"
		}

		fmt.Fprintf(tw, fmtStr,
			prefix,
			fi.Flow.Attributes.SrcIP,
			fi.Flow.Attributes.SrcPort,
			fi.Flow.Attributes.DstIP,
			fi.Flow.Attributes.DstPort,
			protocols.GetIPProto(int(fi.Flow.Attributes.IPProto)),
			fi.Flow.Counters.BytesRcvd,
			fi.Flow.Counters.BytesSent,
			fi.Flow.Counters.PacketsRcvd,
			fi.Flow.Counters.PacketsSent,
		)
	}
	return tw.Flush()
}
