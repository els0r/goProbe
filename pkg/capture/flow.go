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
	ipLayerTypeV4 = 0x04 // IPv4
	ipLayerTypeV6 = 0x06 // IPv6
)

// FlowLog stores flows. It is NOT threadsafe.
type FlowLog struct {
	flowMap map[string]*Flow
}

// NewFlowLog creates a new flow log for storing flows.
func NewFlowLog() *FlowLog {
	return &FlowLog{
		flowMap: make(map[string]*Flow),
	}
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
func (f *FlowLog) Flows() map[string]*Flow {
	return f.flowMap
}

// ParsePacket processes / extracts all information contained in the IP layer received
// from a capture source and converts it to a hash and flags to be added to the flow map
func ParsePacket(ipLayer capture.IPLayer) (epHash capturetypes.EPHash, isIPv4 bool, auxInfo byte, errno capturetypes.ParsingErrno) {

	var protocol byte
	if ipLayerType := ipLayer.Type(); ipLayerType == ipLayerTypeV4 {

		_ = ipLayer[ipv4.HeaderLen-1] // bounds check hint to compiler

		isIPv4, protocol = true, ipLayer[9]

		// Only run the fragmentation checks on fragmented TCP/UDP packets. For
		// ESP, we don't have any transport layer information so there's no
		// need to distinguish between ESP fragments or other ESP traffic
		//
		// Note: an ESP fragment will carry fragmentation information like any
		// other IP packet. The fragment offset will of be MTU - 20 bytes (IP layer).
		if protocol != capturetypes.ESP {

			// Check for IP fragmentation
			fragOffset := (uint16(0x1f&ipLayer[6]) << 8) | uint16(ipLayer[7])

			// Skip packet if it carries anything other than the first fragment,
			// i.e. if the packet lacks a transport layer header
			if fragOffset != 0 {
				errno = capturetypes.ErrnoPacketFragmentIgnore
				return
			}
		}

		// Parse IPv4 packet information
		copy(epHash[0:4], ipLayer[12:16])
		copy(epHash[16:20], ipLayer[16:20])

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
					errno = capturetypes.ErrnoPacketTruncated
					return
				}
				auxInfo = ipLayer[ipv4.HeaderLen+13] // store TCP flags
			}
		} else if protocol == capturetypes.ICMP {
			auxInfo = ipLayer[ipv4.HeaderLen] // store ICMP type
		}
	} else if ipLayerType == ipLayerTypeV6 {

		_ = ipLayer[ipv6.HeaderLen-1] // bounds check hint to compiler

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
					errno = capturetypes.ErrnoPacketTruncated
					return
				}
				auxInfo = ipLayer[ipv6.HeaderLen+13] // store TCP flags
			}
		} else if protocol == capturetypes.ICMPv6 {
			auxInfo = ipLayer[ipv6.HeaderLen] // store ICMP type
		}
	} else {
		errno = capturetypes.ErrnoInvalidIPHeader
		return
	}

	epHash[36] = protocol

	errno = capturetypes.ErrnoOK
	return
}

// Add a packet to the flow log. If the packet belongs to a flow
// already present in the log, the flow will be updated. Otherwise,
// a new flow will be created.
func (f *FlowLog) Add(epHash capturetypes.EPHash, pktType byte, pktSize uint32, isIPv4 bool, auxInfo byte, errno capturetypes.ParsingErrno) capturetypes.ParsingErrno {

	if errno > capturetypes.ErrnoOK {
		if errno.ParsingFailed() {
			return errno
		}
		return capturetypes.ErrnoOK
	}

	// update or assign the flow
	if flowToUpdate, existsHash := f.flowMap[string(epHash[:])]; existsHash {
		flowToUpdate.UpdateFlow(epHash, auxInfo, pktType, pktSize)
	} else {
		epHashReverse := epHash.Reverse()
		if flowToUpdate, existsReverseHash := f.flowMap[string(epHashReverse[:])]; existsReverseHash {
			flowToUpdate.UpdateFlow(epHashReverse, auxInfo, pktType, pktSize)
		} else {
			f.flowMap[string(epHash[:])] = NewFlow(epHash, isIPv4, auxInfo, pktType, pktSize)
		}
	}

	return capturetypes.ErrnoOK
}

// Rotate rotates the flow log. All flows are reset to no packets and traffic.
// Moreover, any flows not worth keeping (according to Flow.IsWorthKeeping)
// are discarded.
//
// Returns an AggFlowMap containing all flows since the last call to Rotate.
func (f *FlowLog) Rotate() (agg *hashmap.AggFlowMap, totals *types.Counters) {
	return f.transferAndAggregate()
}

// Aggregate extracts an AggFlowMap from the currently active flowMap. The flowMap
// itself is not modified in the process.
//
// Returns an AggFlowMap containing all flows since the last call to Rotate.
func (f *FlowLog) Aggregate() (agg *hashmap.AggFlowMap) {

	agg = hashmap.NewAggFlowMap()

	// Reusable key conversion buffers
	keyBufV4, keyBufV6 := types.NewEmptyV4Key(), types.NewEmptyV6Key()
	for _, v := range f.flowMap {

		// Check if the flow actually has any interesting information for us
		if v.packetsRcvd != 0 || v.packetsSent != 0 {

			// Populate key buffer according to source flow
			if v.isIPv4 {
				keyBufV4.PutAllV4(v.epHash[0:4], v.epHash[16:20], v.epHash[32:34], v.epHash[36])
				agg.SetOrUpdate(keyBufV4, v.isIPv4, v.bytesRcvd, v.bytesSent, v.packetsRcvd, v.packetsSent)
			} else {
				keyBufV6.PutAllV6(v.epHash[0:16], v.epHash[16:32], v.epHash[32:34], v.epHash[36])
				agg.SetOrUpdate(keyBufV6, v.isIPv4, v.bytesRcvd, v.bytesSent, v.packetsRcvd, v.packetsSent)
			}
		}
	}

	return
}

func (f *FlowLog) transferAndAggregate() (agg *hashmap.AggFlowMap, totals *types.Counters) {

	// Initialize aggregate flow map / result
	agg = hashmap.NewAggFlowMap()

	// for recomputing the most up to date running sum of bytes and packets
	totals = new(types.Counters)

	// Create reusable key conversion buffers
	keyBufV4, keyBufV6 := types.NewEmptyV4Key(), types.NewEmptyV6Key()

	for k, v := range f.flowMap {

		// Check if the flow actually has any interesting information for us, otherwise
		// delete it from the FlowMap
		if v.packetsRcvd > 0 || v.packetsSent > 0 {
			// update totals
			totals.BytesRcvd += v.bytesRcvd
			totals.BytesSent += v.bytesSent
			totals.PacketsRcvd += v.packetsRcvd
			totals.PacketsSent += v.packetsSent

			// Populate key buffer according to source flow and update result
			if v.isIPv4 {
				keyBufV4.PutAllV4(v.epHash[0:4], v.epHash[16:20], v.epHash[32:34], v.epHash[36])
				agg.SetOrUpdate(keyBufV4, true, v.bytesRcvd, v.bytesSent, v.packetsRcvd, v.packetsSent)
			} else {
				keyBufV6.PutAllV6(v.epHash[0:16], v.epHash[16:32], v.epHash[32:34], v.epHash[36])
				agg.SetOrUpdate(keyBufV6, false, v.bytesRcvd, v.bytesSent, v.packetsRcvd, v.packetsSent)
			}

			// Check whether the flow should be retained / reset for the next interval
			// or thrown away
			if v.directionConfidenceHigh {

				// Reset the flow
				v.Reset()
			} else {
				delete(f.flowMap, k)
			}
		} else {
			delete(f.flowMap, k)
		}
	}

	return
}

func (f *FlowLog) clone() (f2 *FlowLog) {
	f2 = NewFlowLog()
	for k, v := range f.flowMap {
		vCopy := *v
		f2.flowMap[k] = &vCopy
	}
	return
}

// Flow stores a goProbe flow
type Flow struct {
	epHash capturetypes.EPHash

	// Hash Map Value variables
	bytesRcvd               uint64
	bytesSent               uint64
	packetsRcvd             uint64
	packetsSent             uint64
	directionConfidenceHigh bool
	isIPv4                  bool
}

// MarshalJSON implements the Marshaler interface for a flow
func (f *Flow) MarshalJSON() ([]byte, error) {
	return jsoniter.Marshal(f.toExtendedRow())
}

// NewFlow creates a new flow based on the packet
func NewFlow(epHash capturetypes.EPHash, isIPv4 bool, auxInfo byte, pktType capture.PacketType, pktTotalLen uint32) *Flow {

	res := Flow{
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
func (f *Flow) UpdateFlow(epHash capturetypes.EPHash, auxInfo byte, pktType capture.PacketType, pktTotalLen uint32) {

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

// Reset resets all flow counters
func (f *Flow) Reset() {
	f.bytesRcvd = 0
	f.bytesSent = 0
	f.packetsRcvd = 0
	f.packetsSent = 0
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

func (f *Flow) updateDirection(epHash capturetypes.EPHash, auxInfo byte) {
	if direction := capturetypes.ClassifyPacketDirection(epHash, f.isIPv4, auxInfo); direction != capturetypes.DirectionUnknown {
		f.directionConfidenceHigh = direction.IsConfidenceHigh()

		// switch fields if direction was opposite to the default direction
		// "DirectionRemains"
		if direction == capturetypes.DirectionReverts || direction == capturetypes.DirectionMaybeReverts {
			f.epHash = epHash.Reverse()
		}
	}
}

func (f *Flow) toExtendedRow() results.ExtendedRow {
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
