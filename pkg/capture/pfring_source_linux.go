//go:build linux && force_pfring
// +build linux,force_pfring

package capture

import (
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pfring"
)

type PFRingSource struct {
	directions [2]*pfRingDirectionSource
	packets    chan nextPacketData
}

type pfRingDirectionSource struct {
	direction pfring.Direction
	handle    *pfring.Ring
}

func (p *pfRingDirectionSource) init(iface, bpfFilter string, captureLength, bufSize int, promisc bool) error {
	var flags pfring.Flag
	if promisc {
		flags = pfring.FlagPromisc
	}

	handle, err := pfring.NewRing(iface, uint32(captureLength), flags)
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to set up ring: %w", iface, err)
	}

	// we are only interested in reading packets
	err = handle.SetSocketMode(pfring.ReadOnly)
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to set ring to read-only: %w", iface, err)
	}
	err = handle.SetPollDuration(uint(CaptureTimeout / time.Millisecond))
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to set poll timeout to %s: %w", iface, CaptureTimeout, err)
	}
	err = handle.SetBPFFilter(bpfFilter)
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to set bpf filter to %q: %w", iface, bpfFilter, err)
	}
	err = handle.SetDirection(p.direction)
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to set direction to %q: %w", iface, p.direction, err)
	}
	err = handle.Enable()
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to enable PF ring: %w", iface, p.direction, err)
	}
	p.handle = handle

	return nil
}

func (src *pfRingDirectionSource) nextPacket(packets chan<- nextPacketData) {
	npd := nextPacketData{}
	defer func() { packets <- npd }()

	data, ci, err := src.handle.ZeroCopyReadPacketData()
	if err != nil {
		npd.err = err
		return
	}

	packet := gopacket.NewPacket(data, layers.LinkTypeEthernet, defaultDecodeOptions)
	m := packet.Metadata()

	m.CaptureInfo = ci
	m.Truncated = m.Truncated || ci.CaptureLength < ci.Length

	npd.packet = Packet{
		packet:  packet,
		inbound: src.direction == pfring.ReceiveOnly,
	}

	return
}

func (p *PFRingSource) Init(iface, bpfFilter string, captureLength, bufSize int, promisc bool) error {
	p.packets = make(chan nextPacketData, 1024)

	for i, direction := range []pfring.Direction{pfring.ReceiveOnly, pfring.TransmitOnly} {
		ds := &pfRingDirectionSource{direction: direction}

		err := ds.init(iface, bpfFilter, captureLength, bufSize/2, promisc)
		if err != nil {
			return err
		}

		p.directions[i] = ds
	}
	return nil
}

func (p *PFRingSource) NextPacket() (Packet, error) {
	for i := range p.directions {
		go p.directions[i].nextPacket(p.packets)
	}
	npd := <-p.packets
	return npd.packet, npd.err
}

func (p *PFRingSource) Stats() (*CaptureStats, error) {
	var stats = new(CaptureStats)

	for _, direction := range p.directions {
		st, err := direction.handle.Stats()
		if err != nil {
			return nil, err
		}
		add(stats, &CaptureStats{
			PacketsReceived: int(st.Received),
			PacketsDropped:  int(st.Dropped),
		})
	}
	return stats, nil
}

func (p *PFRingSource) Close() {
	for _, direction := range p.directions {
		if direction != nil {
			direction.handle.Close()
		}
	}
}

func (p *PFRingSource) LinkType() gopacket.Decoder {
	return layers.LinkTypeEthernet
}
