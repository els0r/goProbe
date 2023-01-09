package capture

import "github.com/fako1024/gopacket"

var defaultDecodeOptions = gopacket.DecodeOptions{
	Lazy:   true,
	NoCopy: true,
}

type CaptureStats struct {
	PacketsReceived  int
	PacketsDropped   int
	PacketsIfDropped int
}

type Source interface {
	Init(iface, bpfFilter string, captureLength, bufSize int, promisc bool) error
	NextPacket() (gopacket.Packet, error)
	Stats() (*CaptureStats, error)
	LinkType() gopacket.Decoder
	Close()
}
