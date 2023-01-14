package query

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	srcIp1 = netip.MustParseAddr("192.168.1.32")
	srcIp2 = netip.MustParseAddr("192.168.1.62")
	dstIp  = netip.MustParseAddr("8.8.8.8")
)

func TestMerge(t *testing.T) {

	t0 := time.Now()
	t1 := t0.Add(5 * time.Minute)

	var tests = []struct {
		inMap    ResultsMap
		input    Results
		expected Results
	}{
		{
			ResultsMap{
				MergeableAttributes{
					ResultLabels{Timestamp: t0, Iface: "eth0", Hostname: "godzilla"},
					ResultAttributes{DstPort: uint16(443)},
				}: &ResultCounters{BytesReceived: 10, PacketsReceived: 1},
			},
			Results{
				{
					Labels:     ResultLabels{Timestamp: t0, Iface: "eth0", Hostname: "godzilla"},
					Attributes: ResultAttributes{DstPort: uint16(443)},
					Counters:   ResultCounters{BytesReceived: 10, PacketsReceived: 1},
				},
				{
					Labels:     ResultLabels{Timestamp: t1, Iface: "eth0", Hostname: "godzilla"},
					Attributes: ResultAttributes{DstPort: uint16(443)},
					Counters:   ResultCounters{BytesReceived: 10, PacketsReceived: 1},
				},
			},
			Results{
				{
					Labels:     ResultLabels{Timestamp: t0, Iface: "eth0", Hostname: "godzilla"},
					Attributes: ResultAttributes{DstPort: uint16(443)},
					Counters:   ResultCounters{BytesReceived: 20, PacketsReceived: 2},
				},
				{
					Labels:     ResultLabels{Timestamp: t1, Iface: "eth0", Hostname: "godzilla"},
					Attributes: ResultAttributes{DstPort: uint16(443)},
					Counters:   ResultCounters{BytesReceived: 10, PacketsReceived: 1},
				},
			},
		},
		{
			ResultsMap{
				MergeableAttributes{
					ResultLabels{Timestamp: t0, Iface: "eth0", Hostname: "godzilla"},
					ResultAttributes{SrcIP: srcIp1, DstIP: dstIp, IPProto: 6, DstPort: uint16(443)},
				}: &ResultCounters{BytesReceived: 10, PacketsReceived: 1},
			},
			Results{
				{
					Labels:     ResultLabels{Timestamp: t0, Iface: "eth0", Hostname: "godzilla"},
					Attributes: ResultAttributes{SrcIP: srcIp1, DstIP: dstIp, IPProto: 6, DstPort: uint16(443)},
					Counters: ResultCounters{
						BytesReceived: 10, PacketsReceived: 1,
						BytesSent: 10, PacketsSent: 1,
					},
				},
				{
					Labels:     ResultLabels{Timestamp: t0, Iface: "eth0", Hostname: "kong"},
					Attributes: ResultAttributes{SrcIP: srcIp1, DstIP: dstIp, IPProto: 6, DstPort: uint16(443)},
					Counters: ResultCounters{
						BytesReceived: 10, PacketsReceived: 1,
						BytesSent: 10, PacketsSent: 1,
					},
				},
			},
			Results{
				{
					Labels:     ResultLabels{Timestamp: t0, Iface: "eth0", Hostname: "godzilla"},
					Attributes: ResultAttributes{SrcIP: srcIp1, DstIP: dstIp, IPProto: 6, DstPort: uint16(443)},
					Counters: ResultCounters{
						BytesReceived: 20, PacketsReceived: 2,
						BytesSent: 10, PacketsSent: 1,
					},
				},
				{
					Labels:     ResultLabels{Timestamp: t0, Iface: "eth0", Hostname: "kong"},
					Attributes: ResultAttributes{SrcIP: srcIp1, DstIP: dstIp, IPProto: 6, DstPort: uint16(443)},
					Counters: ResultCounters{
						BytesReceived: 10, PacketsReceived: 1,
						BytesSent: 10, PacketsSent: 1,
					},
				},
			},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			test.inMap.MergeResults(test.input)

			out := test.inMap.ToResultsSorted(By(SortTime, DirectionBoth, true))

			assert.Equal(t, test.expected, out)

			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(b))
		})
	}
}
