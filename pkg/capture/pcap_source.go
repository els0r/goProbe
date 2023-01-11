package capture

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

////////////////////////////////////////////////////////

type PcapSource struct {
	directions [2]*directionSource
	packets    chan nextPacketData
}

type directionSource struct {
	direction pcap.Direction
	handle    *pcap.Handle
}

func (src *directionSource) nextPacket(packets chan<- nextPacketData) {
	npd := nextPacketData{}
	defer func() { packets <- npd }()

	// TODO: line is currently commented, since it leads to inconsistencies in captured traffic. Observed when comparing to the
	// stable goProbe version (which uses ReadPacketData) under the hood
	// Needs to be investigated, where the packet data is invalidated. The symptom is: less flows are captured than with ReadPacketData,
	// while the same packet/traffic volume is recorded. Looking at the dominant flows, they match up rather nicely, so the hypothesis
	// is that "data" gets overwritten with one of the next packets,
	// data, ci, err := src.handle.ZeroCopyReadPacketData()
	data, ci, err := src.handle.ReadPacketData()
	if err != nil {
		npd.err = err
		return
	}

	packet := gopacket.NewPacket(data, src.handle.LinkType(), defaultDecodeOptions)
	m := packet.Metadata()

	m.CaptureInfo = ci
	m.Truncated = m.Truncated || ci.CaptureLength < ci.Length

	npd.packet = Packet{
		packet:  packet,
		inbound: src.direction == pcap.DirectionIn,
	}

	return
}

func (p *PcapSource) NextPacket() (Packet, error) {
	for i := range p.directions {
		go p.directions[i].nextPacket(p.packets)
	}
	npd := <-p.packets
	return npd.packet, npd.err
}

func directionToString(direction pcap.Direction) string {
	switch direction {
	case pcap.DirectionIn:
		return "INBOUND"
	case pcap.DirectionOut:
		return "OUTBOUND"
	case pcap.DirectionInOut:
		return "BI-DIRECTIONAL"
	}
	return fmt.Sprintf("pcap.Direction(%d)", direction)
}

func (ds *directionSource) init(iface, bpfFilter string, captureLength, bufSize int, promisc bool) error {
	inactiveHandle, err := setupInactiveHandle(iface, captureLength, bufSize, promisc)
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to create inactive handle for direction %s: %w", iface, directionToString(ds.direction), err)
	}
	defer inactiveHandle.CleanUp()

	PcapMutex.Lock()
	ds.handle, err = inactiveHandle.Activate()
	PcapMutex.Unlock()
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to activate handle for direction %s: %w", iface, directionToString(ds.direction), err)
	}

	// link type might be null if the
	// specified interface does not exist (anymore)
	if ds.handle.LinkType() == layers.LinkTypeNull {
		return fmt.Errorf("Interface '%s': has link type null", iface)
	}

	PcapMutex.Lock()
	err = ds.handle.SetBPFFilter(bpfFilter)
	if err != nil {
		PcapMutex.Unlock()
		return fmt.Errorf("Interface '%s': failed to set bpf filter to %q: %w", iface, bpfFilter, err)
	}
	err = ds.handle.SetDirection(ds.direction)
	PcapMutex.Unlock()
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to set direction to %s: %w", iface, directionToString(ds.direction), err)
	}
	return nil
}

func (p *PcapSource) Init(iface, bpfFilter string, captureLength, bufSize int, promisc bool) error {
	p.packets = make(chan nextPacketData, 1024)

	for i, direction := range []pcap.Direction{pcap.DirectionIn, pcap.DirectionOut} {
		ds := &directionSource{direction: direction}

		// each direction gets half the buffer size specified. This may not accurately reflect the traffic composition, as in there may be more
		// outbound than inbound traffic.
		// TODO: to be considered for future iterations of this code: have some form of learning based on historic traffic patterns as to
		// how the buffer can be accurately sized
		err := ds.init(iface, bpfFilter, captureLength, bufSize/2, promisc)
		if err != nil {
			return err
		}

		p.directions[i] = ds
	}
	return nil
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
		if direction != nil {
			direction.handle.Close()
		}
	}
}

func (p *PcapSource) LinkType() gopacket.Decoder {
	// it's sufficient to return the link type of one of the direction handles since they
	// use the same
	return p.directions[0].handle.LinkType()
}
