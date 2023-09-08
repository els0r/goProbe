package goDB

import (
	"github.com/els0r/goProbe/pkg/formatting"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
)

// InterfaceMetadata describes the time range for which data is available, how many flows
// were recorded and how much traffic was captured
type InterfaceMetadata struct {
	Iface string `json:"iface"`
	results.TimeRange

	gpfile.Stats
}

// TableHeader constructs the table header for pretty printing metadata
func (i *InterfaceMetadata) TableHeader(detailed bool) (headerRows [][]string) {
	r1 := []string{"iface"}
	fromTo := []string{"from", "to"}

	if detailed {
		r0 := []string{"", "packets", "packets", "bytes", "bytes", "# of", "# of", "", "", ""}
		r1 = append(r1, "in", "out", "in", "out", "IPv4 flows", "IPv6 flows", "drops")

		headerRows = append(headerRows, r0)
	} else {
		r1 = append(r1, "packets", "traffic", "flows")
	}

	r1 = append(r1, fromTo...)

	headerRows = append(headerRows, r1)
	return headerRows
}

// TableRow puts all attributes of the metadata into a row that can be used for table printing.
// If detailed is false, the counts and metadata is summarized to their sum (e.g. IPv4 + IPv6 flows = NumFlows).
// Drops are only printed in detail mode
func (i *InterfaceMetadata) TableRow(detailed bool) []string {
	str := []string{i.Iface}
	fromTo := []string{i.First.Format(types.DefaultTimeOutputFormat), i.Last.Format(types.DefaultTimeOutputFormat)}
	if detailed {
		str = append(str,
			formatting.Count(i.Counts.PacketsRcvd), formatting.Count(i.Counts.PacketsSent),
			formatting.Size(i.Counts.BytesRcvd), formatting.Size(i.Counts.BytesSent),
			formatting.Count(i.Traffic.NumV4Entries), formatting.Count(i.Traffic.NumV6Entries),
			formatting.Count(i.Traffic.NumDrops),
		)
	} else {
		str = append(str,
			formatting.Count(i.Counts.PacketsRcvd+i.Counts.PacketsSent),
			formatting.Size(i.Counts.BytesRcvd+i.Counts.BytesSent),
			formatting.Count(i.Traffic.NumFlows()),
		)

	}
	str = append(str, fromTo...)
	return str
}
