package capture

import (
	"fmt"

	"github.com/fako1024/gopacket"
	"github.com/fako1024/gopacket/layers"
	"github.com/fako1024/gopacket/pcap"
)

////////////////////////////////////////////////////////

type PcapSource struct {
	directions [2]*directionSource
}

type directionSource struct {
	bpfPrefix string
	// helper var to save checking everytime if the packet is
	// coming from inbound or outbound traffic
	inbound        uint8
	handle         *pcap.Handle
	nextPacketChan chan *nextPacketData
}

type nextPacketData struct {
	packet gopacket.Packet
	err    error
}

const (
	bpfInbound  = "inbound"
	bpfOutbound = "outbound"
)

func newDirectionSource(bpfPrefix string) *directionSource {
	ds := &directionSource{
		bpfPrefix:      bpfPrefix,
		nextPacketChan: make(chan *nextPacketData, 1),
	}
	if bpfPrefix == bpfInbound {
		ds.inbound = 1
	}
	return ds
}

func (src *directionSource) nextPacket() {
	npd := &nextPacketData{}
	defer func() { src.nextPacketChan <- npd }()

	data, ci, err := src.handle.ZeroCopyReadPacketData()
	if err != nil {
		npd.err = err
		return
	}

	packet := gopacket.NewPacket(data, src.handle.LinkType(), defaultDecodeOptions)
	m := packet.Metadata()

	// set direction for packet
	ci.Inbound = src.inbound

	m.CaptureInfo = ci
	m.Truncated = m.Truncated || ci.CaptureLength < ci.Length

	npd.packet = packet

	return
}

func (p *PcapSource) NextPacket() (gopacket.Packet, error) {

	var npd *nextPacketData

	for _, direction := range p.directions {
		go direction.nextPacket()
	}
	// whichever direction returns first will supply the packet to the caller
	select {
	case npd = <-p.directions[0].nextPacketChan:
		return npd.packet, npd.err
	case npd = <-p.directions[1].nextPacketChan:
		return npd.packet, npd.err
	}
}

func (p *PcapSource) Init(iface, bpfFilter string, captureLength, bufSize int, promisc bool) error {

	for i, direction := range []string{bpfInbound, bpfOutbound} {
		ds := newDirectionSource(direction)

		// each direction gets half the buffer size specified. This may not accurately reflect the traffic composition, as in there may be more
		// outbound than inbound traffic.
		// TODO: to be considered for future iterations of this code: have some form of learning based on historic traffic patterns as to
		// how the buffer can be accurately sized
		inactiveHandle, err := setupInactiveHandle(iface, captureLength, bufSize/2, promisc)
		if err != nil {
			return fmt.Errorf("Interface '%s': failed to create inactive handle for direction %q: %w", iface, direction, err)
		}
		defer inactiveHandle.CleanUp()

		PcapMutex.Lock()
		ds.handle, err = inactiveHandle.Activate()
		PcapMutex.Unlock()
		if err != nil {
			return fmt.Errorf("Interface '%s': failed to activate handle for direction %q: %w", iface, direction, err)
		}

		// link type might be null if the
		// specified interface does not exist (anymore)
		if ds.handle.LinkType() == layers.LinkTypeNull {
			return fmt.Errorf("Interface '%s': has link type null", iface)
		}

		// prepend direction bpf filter
		dirBpfFilter := fmt.Sprintf("%s and (%s)", direction, bpfFilter)

		PcapMutex.Lock()
		// TODO: currently fails because of https://github.com/nmap/npcap/issues/403
		// more info: https://tcpdump-workers.tcpdump.narkive.com/HVicNWd4/relation-of-pcap-setdirection-and-inbound-outbound-filter-qualifiers
		err = ds.handle.SetBPFFilter(dirBpfFilter)
		PcapMutex.Unlock()
		if err != nil {
			return fmt.Errorf("Interface '%s': failed to set bpf filter to %s: %w", iface, dirBpfFilter, err)
		}
		p.directions[i] = ds
	}

	return nil
}

// add is a convenience method to total capture stats. This is relevant in the scope of
// adding statistics from the two directions. The result of the addition is written back
// to a to reduce allocations
func add(a, b *CaptureStats) {
	a.PacketsReceived += b.PacketsReceived
	a.PacketsDropped += b.PacketsDropped
	a.PacketsIfDropped += b.PacketsIfDropped
}

func (p *PcapSource) Stats() (*CaptureStats, error) {
	var stats = new(CaptureStats)

	for _, direction := range p.directions {
		st, err := direction.handle.Stats()
		if err != nil {
			return nil, err
		}
		add(stats, &CaptureStats{
			PacketsReceived:  st.PacketsReceived,
			PacketsDropped:   st.PacketsDropped,
			PacketsIfDropped: st.PacketsIfDropped,
		})
	}
	return stats, nil
}

func (p *PcapSource) Close() {
	for _, direction := range p.directions {
		direction.handle.Close()
	}
}

func (p *PcapSource) LinkType() gopacket.Decoder {
	// it's sufficient to return the link type of one of the direction handles since they
	// use the same
	return p.directions[0].handle.LinkType()
}
