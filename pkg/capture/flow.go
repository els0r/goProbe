package capture

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

	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/goDB/protocols"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/fako1024/slimcap/capture"
	jsoniter "github.com/json-iterator/go"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	ipLayerTypeV4 = 0x04 // 4
	ipLayerTypeV6 = 0x06 // 6
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

// ParsePacket processes / extracts all information contained in the IP layer received
// from a capture source and converts it to a hash and flags to be added to the flow map
func ParsePacket(ipLayer capture.IPLayer, pktType capture.PacketType, pktTotalLen uint32) (epHash capturetypes.EPHash, isIPv4 bool, auxInfo byte, err error) {

	var protocol byte
	if ipLayerType := ipLayer.Type(); ipLayerType == ipLayerTypeV4 {

		isIPv4, protocol = true, ipLayer[9]

		// Parse IPv4 packet information
		copy(epHash[0:4], ipLayer[12:16])
		copy(epHash[16:20], ipLayer[16:20])

		// only run the fragmentation checks on fragmented TCP/UDP packets. For
		// ESP, we don't have any transport layer information so there's no
		// need to distinguish between ESP fragments or other ESP traffic
		//
		// Note: an ESP fragment will carry fragmentation information like any
		// other IP packet. The fragment offset will of be MTU - 20 bytes (IP layer).
		if protocol != capturetypes.ESP {

			// check for IP fragmentation
			fragOffset := (uint16(0x1f&ipLayer[6]) << 8) | uint16(ipLayer[7])

			// return decoding error if the packet carries anything other than the
			// first fragment, i.e. if the packet lacks a transport layer header
			if fragOffset != 0 {
				fragBits := (0xe0 & ipLayer[6]) >> 5
				err = fmt.Errorf("fragmented IP packet: offset: %d flags: %d", fragOffset, fragBits)
				return
			}
		}

		if protocol == capturetypes.TCP || protocol == capturetypes.UDP {

			dport := ipLayer[ipv4.HeaderLen+2 : ipv4.HeaderLen+4]
			sport := ipLayer[ipv4.HeaderLen : ipv4.HeaderLen+2]

			// If session based traffic is observed, the source port is taken
			// into account. A major exception is traffic over port 53 as
			// considering every single DNS request/response would
			// significantly fill up the flow map
			if !isCommonPort(dport, protocol) {
				copy(epHash[34:36], sport)
			}
			if !isCommonPort(sport, protocol) {
				copy(epHash[32:34], dport)
			}

			if protocol == capturetypes.TCP {
				if len(ipLayer) < ipv4.HeaderLen+13 {
					err = fmt.Errorf("tcp packet too short (len %d)", len(ipLayer))
					return
				}
				auxInfo = ipLayer[ipv4.HeaderLen+13] // store TCP flags
			}
		} else if protocol == capturetypes.ICMP {
			auxInfo = ipLayer[ipv4.HeaderLen] // store ICMP type
		}

	} else if ipLayerType == ipLayerTypeV6 {

		protocol = ipLayer[6]

		// Parse IPv6 packet information
		copy(epHash[0:16], ipLayer[8:24])
		copy(epHash[16:32], ipLayer[24:40])

		if protocol == capturetypes.TCP || protocol == capturetypes.UDP {

			dport := ipLayer[ipv6.HeaderLen+2 : ipv6.HeaderLen+4]
			sport := ipLayer[ipv6.HeaderLen : ipv6.HeaderLen+2]

			// If session based traffic is observed, the source port is taken
			// into account. A major exception is traffic over port 53 as
			// considering every single DNS request/response would
			// significantly fill up the flow map
			if !isCommonPort(dport, protocol) {
				copy(epHash[34:36], sport)
			}
			if !isCommonPort(sport, protocol) {
				copy(epHash[32:34], dport)
			}

			if protocol == capturetypes.TCP {
				if len(ipLayer) < ipv6.HeaderLen+13 {
					err = fmt.Errorf("tcp packet too short (len %d)", len(ipLayer))
					return
				}
				auxInfo = ipLayer[ipv6.HeaderLen+13] // store TCP flags
			}
		} else if protocol == capturetypes.ICMPv6 {
			auxInfo = ipLayer[ipv6.HeaderLen] // store ICMP type
		}

	} else {
		err = fmt.Errorf("received neither IPv4 nor IPv6 IP header: %v", ipLayer)
		return
	}

	epHash[36] = protocol

	return
}

// Add a packet to the flow log. If the packet belongs to a flow
// already present in the log, the flow will be updated. Otherwise,
// a new flow will be created.
func (f *FlowLog) Add(ipLayer capture.IPLayer, pktType capture.PacketType, pktTotalLen uint32) error {

	epHash, isIPv4, auxInfo, err := ParsePacket(ipLayer, pktType, pktTotalLen)
	if err != nil {
		return err
	}

	// update or assign the flow
	if flowToUpdate, existsHash := f.flowMap[string(epHash[:])]; existsHash {
		flowToUpdate.UpdateFlow(epHash, auxInfo, pktType, pktTotalLen)
	} else {
		epHashReverse := epHash.Reverse()
		if flowToUpdate, existsReverseHash := f.flowMap[string(epHashReverse[:])]; existsReverseHash {
			flowToUpdate.UpdateFlow(epHashReverse, auxInfo, pktType, pktTotalLen)
		} else {
			f.flowMap[string(epHash[:])] = NewGPFlow(epHash, isIPv4, auxInfo, pktType, pktTotalLen)
		}
	}

	return nil
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

// GPFlow stores a goProbe flow
type GPFlow struct {
	epHash capturetypes.EPHash

	// Hash Map Value variables
	bytesRcvd               uint64
	bytesSent               uint64
	packetsRcvd             uint64
	packetsSent             uint64
	directionConfidenceHigh bool
	isIPv4                  bool
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
	return jsoniter.Marshal(f.toExtendedRow())
}

// NewGPFlow creates a new flow based on the packet
func NewGPFlow(epHash capturetypes.EPHash, isIPv4 bool, auxInfo byte, pktType capture.PacketType, pktTotalLen uint32) *GPFlow {

	res := GPFlow{
		epHash: epHash,
		isIPv4: isIPv4,
	}
	res.updateDirection(epHash, auxInfo)

	// set packet and byte counters with respect to its interface direction
	if pktType != capture.PacketOutgoing {
		res.bytesRcvd = uint64(pktTotalLen)
		res.packetsRcvd = 1
	} else {
		res.bytesSent = uint64(pktTotalLen)
		res.packetsSent = 1
	}

	return &res
}

// UpdateFlow increments flow counters if the packet belongs to an existing flow
func (f *GPFlow) UpdateFlow(epHash capturetypes.EPHash, auxInfo byte, pktType capture.PacketType, pktTotalLen uint32) {

	// increment packet and byte counters with respect to its interface direction
	if pktType != capture.PacketOutgoing {
		f.bytesRcvd += uint64(pktTotalLen)
		f.packetsRcvd++
	} else {
		f.bytesSent += uint64(pktTotalLen)
		f.packetsSent++
	}

	// try to update direction if necessary (as long as we're not confident enough)
	if !f.directionConfidenceHigh {
		f.updateDirection(epHash, auxInfo)
	}
}

// IsWorthKeeping is used by a flow to check whether it has any interesting direction into
// worth keeping and whether its counters are non-zero. If they are, it means that
// the flow was essentially idle in the last time interval and that it can be safely
// discarded.
func (f *GPFlow) IsWorthKeeping() bool {
	return f.directionConfidenceHigh && !f.HasBeenIdle()
}

// Reset resets all flow counters
func (f *GPFlow) Reset() {
	f.bytesRcvd = 0
	f.bytesSent = 0
	f.packetsRcvd = 0
	f.packetsSent = 0
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

func (f *GPFlow) updateDirection(epHash capturetypes.EPHash, auxInfo byte) {
	if direction := capturetypes.ClassifyPacketDirection(epHash, f.isIPv4, auxInfo); direction != capturetypes.DirectionUnknown {
		f.directionConfidenceHigh = direction.IsConfidenceHigh()

		// switch fields if direction was opposite to the default direction
		// "DirectionRemains"
		if direction == capturetypes.DirectionReverts || direction == capturetypes.DirectionMaybeReverts {
			f.epHash = epHash.Reverse()
		}
	}
}

func (f *GPFlow) toExtendedRow() results.ExtendedRow {
	return results.ExtendedRow{
		Attributes: results.ExtendedAttributes{
			SrcPort: types.PortToUint16(f.epHash[34:36]),
			Attributes: results.Attributes{
				SrcIP:   types.RawIPToAddr(f.epHash[0:16]),
				DstIP:   types.RawIPToAddr(f.epHash[16:32]),
				DstPort: types.PortToUint16(f.epHash[32:34]),
				IPProto: f.epHash[36],
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

func isCommonPort(port []byte, proto byte) bool {
	// Fast path for neither of the below
	if port[0] > 1 {
		return false
	}

	// TCP common ports
	if proto == capturetypes.TCP {
		return (port[0] == 0 && (port[1] == 53 || port[1] == 80)) || // DNS(TCP), HTTP
			(port[0] == 1 && port[1] == 187) // HTTPS
	}

	// UDP common ports
	if proto == capturetypes.UDP {
		return (port[0] == 0 && port[1] == 53) || // DNS(UDP)
			(port[0] == 1 && port[1] == 187) // 443(UDP)
	}

	return false
}
