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
