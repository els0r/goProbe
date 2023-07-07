package goDB

import (
	"github.com/els0r/goProbe/pkg/formatting"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
)

// CaptureMetadata represents metadata for one database block.
// TODO: This will be replaced by global Capture Stats when merging #47
// TODO: check with fako1024 if this can be removed already or if this is
// post-v4-release
type CaptureMetadata struct {
	PacketsDropped int
}

// InterfaceMetadata describes the time range for which data is available, how many flows
// were recorded and how much traffic was captured
type InterfaceMetadata struct {
	Iface string `json:"iface"`
	results.TimeRange

	gpfile.Stats
}

// TableHeader constructs the table header for pretty printing metadata
func (i *InterfaceMetadata) TableHeader(detailed bool) [2][]string {
	r0 := []string{"", "", ""}
	r1 := []string{"iface", "from", "to"}
	if detailed {
		r0 = append(r0, "bytes", "bytes", "packets", "packets", "# IPv4", "# IPv6", "")
		r1 = append(r1, "received", "sent", "received", "sent", "flows", "flows", "drops")
	} else {
		r0 = append(r0, "bytes", "packets", "#")
		r1 = append(r1, "sent+rcvd", "sent+rcvd", "flows")
	}
	return [2][]string{r0, r1}
}

// TableRow puts all attributes of the metadata into a row that can be used for table printing.
// If detailed is false, the counts and metadata is summarized to their sum (e.g. IPv4 + IPv6 flows = NumFlows).
// Drops are only printed in detail mode
func (i *InterfaceMetadata) TableRow(detailed bool) []string {
	str := []string{i.Iface, i.First.Format(types.DefaultTimeOutputFormat), i.Last.Format(types.DefaultTimeOutputFormat)}
	if detailed {
		str = append(str,
			formatting.Size(i.Counts.BytesRcvd), formatting.Size(i.Counts.BytesSent),
			formatting.Count(i.Counts.PacketsRcvd), formatting.Size(i.Counts.PacketsSent),
			formatting.Count(i.Traffic.NumV4Entries), formatting.Size(i.Traffic.NumV6Entries),
			formatting.Count(uint64(i.Traffic.NumDrops)),
		)
	} else {
		str = append(str,
			formatting.Size(i.Counts.BytesRcvd+i.Counts.BytesSent),
			formatting.Count(i.Counts.PacketsRcvd+i.Counts.PacketsSent),
			formatting.Count(i.Traffic.NumFlows()),
		)

	}
	return str
}
