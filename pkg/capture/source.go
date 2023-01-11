package capture

import "github.com/google/gopacket"

var defaultDecodeOptions = gopacket.DecodeOptions{
	Lazy:   true,
	NoCopy: true,
}

type CaptureStats struct {
	PacketsReceived  int
	PacketsDropped   int
	PacketsIfDropped int
}

type nextPacketData struct {
	packet Packet
	err    error
}

type Packet struct {
	packet  gopacket.Packet
	inbound bool
}

type Source interface {
	Init(iface, bpfFilter string, captureLength, bufSize int, promisc bool) error
	NextPacket() (Packet, error)
	Stats() (*CaptureStats, error)
	LinkType() gopacket.Decoder
	Close()
}

// add is a convenience method to total capture stats. This is relevant in the scope of
// adding statistics from the two directions. The result of the addition is written back
// to a to reduce allocations
func add(a, b *CaptureStats) {
	a.PacketsReceived += b.PacketsReceived
	a.PacketsDropped += b.PacketsDropped
	a.PacketsIfDropped += b.PacketsIfDropped
}
