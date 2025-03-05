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

	"github.com/els0r/goProbe/v4/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/v4/pkg/goDB/protocols"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/goProbe/v4/pkg/types/hashmap"
	"github.com/fako1024/slimcap/capture"
	jsoniter "github.com/json-iterator/go"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	ipLayerTypeV4 = 0x04 // IPv4
	ipLayerTypeV6 = 0x06 // IPv6

	ipLayerV4ProtoPos          = 9
	ipLayerV4SipStart          = 12
	ipLayerV4SipEnd            = 16
	ipLayerV4DipStart          = 16
	ipLayerV4DipEnd            = 20
	ipLayerV4SPortStart        = ipv4.HeaderLen
	ipLayerV4SPortEnd          = ipv4.HeaderLen + 2
	ipLayerV4DPortStart        = ipv4.HeaderLen + 2
	ipLayerV4DPortEnd          = ipv4.HeaderLen + 4
	ipLayerV4TCPFlagsPos       = ipv4.HeaderLen + 13
	ipLayerV4FragFlagFirstByte = 6
	ipLayerV4FragFlagLastByte  = 7

	ipLayerV6BoundsLimit = ipv6.HeaderLen - 1
	ipLayerV6ProtoPos    = 6
	ipLayerV6SipStart    = 8
	ipLayerV6SipEnd      = 24
	ipLayerV6DipStart    = 24
	ipLayerV6DipEnd      = 40
	ipLayerV6SPortStart  = ipv6.HeaderLen
	ipLayerV6SPortEnd    = ipv6.HeaderLen + 2
	ipLayerV6DPortStart  = ipv6.HeaderLen + 2
	ipLayerV6DPortEnd    = ipv6.HeaderLen + 4
	ipLayerV6TCPFlagsPos = ipv6.HeaderLen + 13

	ipLayerV4BoundsLimit = ipv4.HeaderLen - 1
	ipLayerV4TCPLimit    = ipLayerV4TCPFlagsPos + 1
	ipLayerV4UDPLimit    = ipLayerV4DPortEnd
	ipLayerV4ICMPLimit   = ipv4.HeaderLen + 1
	ipLayerV6TCPLimit    = ipLayerV6TCPFlagsPos + 1
	ipLayerV6UDPLimit    = ipLayerV6DPortEnd
	ipLayerV6ICMPLimit   = ipv6.HeaderLen + 1
)

// FlowLog stores flows. It is NOT threadsafe.
type FlowLog struct {
	flowMapV4 map[string]*Flow
	flowMapV6 map[string]*Flow
}

// NewFlowLog creates a new flow log for storing flows.
func NewFlowLog() *FlowLog {
	return &FlowLog{
		flowMapV4: make(map[string]*Flow),
		flowMapV6: make(map[string]*Flow),
	}
}

// MarshalJSON implements the jsoniter.Marshaler interface
func (f *FlowLog) MarshalJSON() ([]byte, error) {
	var toMarshal []interface{}
	for _, v := range f.flowMapV4 {
		toMarshal = append(toMarshal, v)
	}
	for _, v := range f.flowMapV6 {
		toMarshal = append(toMarshal, v)
	}
	return jsoniter.Marshal(toMarshal)
}

// Len returns the number of flows in the FlowLog
func (f *FlowLog) Len() int {
	return len(f.flowMapV4) + len(f.flowMapV6)
}

// FlowsV4 provides an iterator for the internal flow map
func (f *FlowLog) FlowsV4() map[string]*Flow {
	return f.flowMapV4
}

// FlowsV6 provides an iterator for the internal flow map
func (f *FlowLog) FlowsV6() map[string]*Flow {
	return f.flowMapV6
}

// ParsePacketV4 processes / extracts all information contained in the v6 IP layer received
// from a capture source and converts it to a hash and flags to be added to the flow map
func ParsePacketV4(ipLayer capture.IPLayer) (epHash capturetypes.EPHashV4, auxInfo byte, errno capturetypes.ParsingErrno) {

	_ = ipLayer[ipLayerV4BoundsLimit] // bounds check hint to compiler
	protocol := ipLayer[ipLayerV4ProtoPos]

	// Only run the fragmentation checks on fragmented TCP/UDP packets. For
	// ESP, we don't have any transport layer information so there's no
	// need to distinguish between ESP fragments or other ESP traffic
	//
	// Note: an ESP fragment will carry fragmentation information like any
	// other IP packet. The fragment offset will of be MTU - 20 bytes (IP layer).
	if protocol != capturetypes.ESP {

		// Check for IP fragmentation
		fragOffset := (uint16(0x1f&ipLayer[ipLayerV4FragFlagFirstByte]) << 8) | uint16(ipLayer[ipLayerV4FragFlagLastByte])

		// Skip packet if it carries anything other than the first fragment,
		// i.e. if the packet lacks a transport layer header
		if fragOffset != 0 {
			errno = capturetypes.ErrnoPacketFragmentIgnore
			return
		}
	}

	// Parse source / destination IPs
	copy(epHash[capturetypes.EPHashV4SipStart:capturetypes.EPHashV4SipEnd], ipLayer[ipLayerV4SipStart:ipLayerV4SipEnd])
	copy(epHash[capturetypes.EPHashV4DipStart:capturetypes.EPHashV4DipEnd], ipLayer[ipLayerV4DipStart:ipLayerV4DipEnd])

	var dport, sport []byte

	// Parse TCP protocol
	if protocol == capturetypes.TCP {
		if len(ipLayer) < ipLayerV4TCPLimit {
			errno = capturetypes.ErrnoPacketTruncated
			return
		}

		auxInfo = ipLayer[ipLayerV4TCPFlagsPos] // store TCP flags

		goto ports
	}

	// Parse UDP protocol
	if protocol == capturetypes.UDP {
		if len(ipLayer) < ipLayerV4UDPLimit {
			errno = capturetypes.ErrnoPacketTruncated
			return
		}

		goto ports
	}

	// Parse ICMP protocol
	if protocol == capturetypes.ICMP {
		if len(ipLayer) < ipLayerV4ICMPLimit {
			errno = capturetypes.ErrnoPacketTruncated
			return
		}

		auxInfo = ipLayer[ipv4.HeaderLen] // store ICMP type
	}
	goto finalize

ports:
	dport = ipLayer[ipLayerV4DPortStart:ipLayerV4DPortEnd]
	sport = ipLayer[ipLayerV4SPortStart:ipLayerV4SPortEnd]

	// If session based traffic is observed, the source port is taken
	// into account. A major exception is traffic over port 53 as
	// considering every single DNS request/response would
	// significantly fill up the flow map
	if !isCommonPort(dport, protocol) {
		copy(epHash[capturetypes.EPHashV4SPortStart:capturetypes.EPHashV4SPortEnd], sport)
	}
	if !isCommonPort(sport, protocol) {
		copy(epHash[capturetypes.EPHashV4DPortStart:capturetypes.EPHashV4DPortEnd], dport)
	}

finalize:
	epHash[capturetypes.EPHashV4ProtocolPos] = protocol

	errno = capturetypes.ErrnoOK
	return
}

// ParsePacketV6 processes / extracts all information contained in the v6 IP layer received
// from a capture source and converts it to a hash and flags to be added to the flow map
func ParsePacketV6(ipLayer capture.IPLayer) (epHash capturetypes.EPHashV6, auxInfo byte, errno capturetypes.ParsingErrno) {

	_ = ipLayer[ipLayerV6BoundsLimit] // bounds check hint to compiler
	protocol := ipLayer[ipLayerV6ProtoPos]

	// Parse source / destination IPs
	copy(epHash[capturetypes.EPHashV6SipStart:capturetypes.EPHashV6SipEnd], ipLayer[ipLayerV6SipStart:ipLayerV6SipEnd])
	copy(epHash[capturetypes.EPHashV6DipStart:capturetypes.EPHashV6DipEnd], ipLayer[ipLayerV6DipStart:ipLayerV6DipEnd])

	var dport, sport []byte

	// Parse TCP protocol
	if protocol == capturetypes.TCP {
		if len(ipLayer) < ipLayerV6TCPLimit {
			errno = capturetypes.ErrnoPacketTruncated
			return
		}
		auxInfo = ipLayer[ipLayerV6TCPFlagsPos] // store TCP flags

		goto ports
	}

	// Parse UDP protocol
	if protocol == capturetypes.UDP {
		if len(ipLayer) < ipLayerV6UDPLimit {
			errno = capturetypes.ErrnoPacketTruncated
			return
		}

		goto ports
	}

	// Parse ICMP protocol
	if protocol == capturetypes.ICMPv6 {
		if len(ipLayer) < ipLayerV6ICMPLimit {
			errno = capturetypes.ErrnoPacketTruncated
			return
		}

		auxInfo = ipLayer[ipv6.HeaderLen] // store ICMP type
	}
	goto finalize

ports:
	dport = ipLayer[ipLayerV6DPortStart:ipLayerV6DPortEnd]
	sport = ipLayer[ipLayerV6SPortStart:ipLayerV6SPortEnd]

	// If session based traffic is observed, the source port is taken
	// into account. A major exception is traffic over port 53 as
	// considering every single DNS request/response would
	// significantly fill up the flow map
	if !isCommonPort(dport, protocol) {
		copy(epHash[capturetypes.EPHashV6SPortStart:capturetypes.EPHashV6SPortEnd], sport)
	}
	if !isCommonPort(sport, protocol) {
		copy(epHash[capturetypes.EPHashV6DPortStart:capturetypes.EPHashV6DPortEnd], dport)
	}

finalize:
	epHash[capturetypes.EPHashV6ProtocolPos] = protocol

	errno = capturetypes.ErrnoOK
	return
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

	// Initialize aggregate flow map / result
	agg = hashmap.NewAggFlowMap()

	// Reusable key conversion buffers
	keyBufV4, keyBufV6 := types.NewEmptyV4Key(), types.NewEmptyV6Key()
	for k, v := range f.flowMapV4 {

		// Check if the flow actually has any interesting information for us
		if v.PacketsRcvd != 0 || v.PacketsSent != 0 {

			// Populate key buffer according to source flow
			keyBufV4.PutV4String(k)
			agg.PrimaryMap.SetOrUpdate(keyBufV4, v.BytesRcvd, v.BytesSent, v.PacketsRcvd, v.PacketsSent)
		}
	}

	for k, v := range f.flowMapV6 {

		// Check if the flow actually has any interesting information for us
		if v.PacketsRcvd != 0 || v.PacketsSent != 0 {

			// Populate key buffer according to source flow
			keyBufV6.PutV6String(k)
			agg.SecondaryMap.SetOrUpdate(keyBufV6, v.BytesRcvd, v.BytesSent, v.PacketsRcvd, v.PacketsSent)
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

	for k, v := range f.flowMapV4 {

		// Check if the flow actually has any interesting information for us, otherwise
		// delete it from the FlowMap
		if v.PacketsRcvd > 0 || v.PacketsSent > 0 {

			// Update totals
			totals.Add(types.Counters(*v))

			// Populate key buffer according to source flow and update result
			keyBufV4.PutV4String(k)
			agg.PrimaryMap.SetOrUpdate(keyBufV4, v.BytesRcvd, v.BytesSent, v.PacketsRcvd, v.PacketsSent)

			// Reset the flow
			v.Reset()
			continue
		}

		delete(f.flowMapV4, k)
	}

	for k, v := range f.flowMapV6 {

		// Check if the flow actually has any interesting information for us, otherwise
		// delete it from the FlowMap
		if v.PacketsRcvd > 0 || v.PacketsSent > 0 {

			// Update totals
			totals.Add(types.Counters(*v))

			// Populate key buffer according to source flow and update result
			keyBufV6.PutV6String(k)
			agg.SecondaryMap.SetOrUpdate(keyBufV6, v.BytesRcvd, v.BytesSent, v.PacketsRcvd, v.PacketsSent)

			// Reset the flow
			v.Reset()
			continue
		}

		delete(f.flowMapV6, k)

	}

	return
}

func (f *FlowLog) clone() (f2 *FlowLog) {
	f2 = NewFlowLog()
	for k, v := range f.flowMapV4 {
		vCopy := *v
		f2.flowMapV4[k] = &vCopy
	}
	for k, v := range f.flowMapV6 {
		vCopy := *v
		f2.flowMapV6[k] = &vCopy
	}
	return
}

// Flow stores a goProbe flow (alias for types.Counters to allow for extension with
// flow specific methods)
type Flow types.Counters

// NewFlow creates a new flow based on the packet
func NewFlow(pktType capture.PacketType, pktTotalLen uint32) *Flow {

	// Set packet and byte counters with respect to the interface direction
	if pktType == capture.PacketOutgoing {
		return &Flow{
			BytesSent:   uint64(pktTotalLen),
			PacketsSent: 1,
		}
	}

	return &Flow{
		BytesRcvd:   uint64(pktTotalLen),
		PacketsRcvd: 1,
	}
}

// UpdateFlow increments flow counters if the packet belongs to an existing flow
func (f *Flow) UpdateFlow(pktType capture.PacketType, pktTotalLen uint32) {

	// increment packet and byte counters with respect to its interface direction
	if pktType == capture.PacketOutgoing {
		f.BytesSent += uint64(pktTotalLen)
		f.PacketsSent++
		return
	}

	f.BytesRcvd += uint64(pktTotalLen)
	f.PacketsRcvd++
}

// Reset resets / null all counter values
func (f *Flow) Reset() {
	f.BytesRcvd = 0
	f.BytesSent = 0
	f.PacketsRcvd = 0
	f.PacketsSent = 0
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

const commonPortsMaxTrackedFirstByte = 31

// Byte-level lookup table for common ports (allows for constant-time lookup that is almost as
// fast as a best case conditional logic)
var commonPorts = [18][commonPortsMaxTrackedFirstByte + 1][256]bool{

	// TCP
	6: {
		0: {
			53: true, // 53/TCP (DNS)
			80: true, // 80/TCP (HTTP)
		},
		1: {
			187: true, // 443/TCP (HTTPS)
			189: true, // 445/TCP (SMB)
		},
		31: {
			144: true, // 8080/TCP (Proxy)
		},
	},

	// UDP
	17: {
		0: {
			53: true, // 53/UDP (DNS)
		},
		1: {
			187: true, // 443/UDP (streaming etc.)
		},
	},
}

func isCommonPort(port []byte, proto byte) bool {

	// Fast path for unsupported protocols / obvious cases
	if port[0] > commonPortsMaxTrackedFirstByte || proto > capturetypes.UDP {
		return false
	}

	return commonPorts[proto][port[0]][port[1]]
}
