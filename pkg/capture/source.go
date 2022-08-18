package capture

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fako1024/gopacket"
	"github.com/fako1024/gopacket/afpacket"
	"github.com/fako1024/gopacket/layers"
	"github.com/fako1024/gopacket/pcap"
	"golang.org/x/net/bpf"
)

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
	NextPacket() (gopacket.Packet, error)

	Init(iface, bpfFilter string, captureLength, bufSize int, promisc bool) error

	Stats() (*CaptureStats, error)

	Close()

	LinkType() gopacket.Decoder
}

////////////////////////////////////////////////////////

type PcapSource struct {
	handle *pcap.Handle
}

func (p *PcapSource) NextPacket() (gopacket.Packet, error) {
	data, ci, err := p.handle.ZeroCopyReadPacketData()
	if err != nil {
		return nil, err
	}

	packet := gopacket.NewPacket(data, p.handle.LinkType(), defaultDecodeOptions)
	m := packet.Metadata()
	m.CaptureInfo = ci
	m.Truncated = m.Truncated || ci.CaptureLength < ci.Length

	return packet, nil
}

func (p *PcapSource) Init(iface, bpfFilter string, captureLength, bufSize int, promisc bool) error {

	inactiveHandle, err := setupInactiveHandle(iface, captureLength, bufSize, promisc)
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to create inactive handle: %w", iface, err)
	}
	defer inactiveHandle.CleanUp()

	PcapMutex.Lock()
	p.handle, err = inactiveHandle.Activate()
	PcapMutex.Unlock()
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to activate handle: %w", iface, err)
	}

	// link type might be null if the
	// specified interface does not exist (anymore)
	if p.handle.LinkType() == layers.LinkTypeNull {
		return fmt.Errorf("Interface '%s': has link type null", iface)
	}

	PcapMutex.Lock()
	err = p.handle.SetBPFFilter(bpfFilter)
	PcapMutex.Unlock()
	if err != nil {
		return fmt.Errorf("Interface '%s': failed to set bpf filter to %s: %w", iface, bpfFilter, err)
	}

	return nil
}

func (p *PcapSource) Stats() (*CaptureStats, error) {
	stats, err := p.handle.Stats()
	if err != nil {
		return nil, err
	}
	return &CaptureStats{
		PacketsReceived:  stats.PacketsReceived,
		PacketsDropped:   stats.PacketsDropped,
		PacketsIfDropped: stats.PacketsIfDropped,
	}, nil
}

func (p *PcapSource) Close() {
	p.handle.Close()
}

func (p *PcapSource) LinkType() gopacket.Decoder {
	return p.handle.LinkType()
}

////////////////////////////////////////////////////////

type AFPacketSource struct {
	handle *afpacket.TPacket
}

func (p *AFPacketSource) NextPacket() (gopacket.Packet, error) {
	data, ci, err := p.handle.ZeroCopyReadPacketData()
	if err != nil {
		return nil, err
	}

	packet := gopacket.NewPacket(data, layers.LinkTypeEthernet, defaultDecodeOptions)
	m := packet.Metadata()
	m.CaptureInfo = ci
	m.Truncated = m.Truncated || ci.CaptureLength < ci.Length

	// fmt.Println("GOT AF_PACKET", packet.String())

	return packet, nil
}

func (p *AFPacketSource) Init(iface, bpfFilter string, captureLength, bufSize int, promisc bool) error {

	if captureLength <= 0 {
		captureLength = 65535
	}

	szFrame, szBlock, numBlocks, err := afpacketComputeSize(bufSize, captureLength, os.Getpagesize())
	if err != nil {
		return err
	}

	p.handle, err = newAfpacketHandle(iface, szFrame, szBlock, numBlocks, false, pcap.BlockForever)
	if err != nil {
		return err
	}

	// TODO: This is unfortunate. The AF_PACKET capture mode is supposed to work without CGO
	// dependency. However, to set it up we have to depend on the BPF filtering, which is <drumroll>
	// part of the pcap package (which in turn depends on CGO)...
	pcapBPF, err := pcap.CompileBPFFilter(layers.LinkTypeEthernet, captureLength, bpfFilter)
	if err != nil {
		return err
	}

	bpfIns := []bpf.RawInstruction{}
	for _, ins := range pcapBPF {
		bpfIns2 := bpf.RawInstruction{
			Op: ins.Code,
			Jt: ins.Jt,
			Jf: ins.Jf,
			K:  ins.K,
		}
		bpfIns = append(bpfIns, bpfIns2)
	}

	return p.handle.SetBPF(bpfIns)
}

func (p *AFPacketSource) Stats() (*CaptureStats, error) {
	_, stats, err := p.handle.SocketStats()
	if err != nil {
		return nil, err
	}
	return &CaptureStats{
		PacketsReceived: int(stats.Packets()),
		PacketsDropped:  int(stats.Drops()),
	}, nil
}

func (p *AFPacketSource) Close() {
	p.handle.Close()
}

func (p *AFPacketSource) LinkType() gopacket.Decoder {
	return layers.LinkTypeEthernet
}

// afpacketComputeSize computes the block_size and the num_blocks in such a way that the
// allocated mmap buffer is close to but smaller than target_size_mb.
// The restriction is that the block_size must be divisible by both the
// frame size and page size.
// TODO: This seems off and needs improvement for the used low SnapLen of 86
func afpacketComputeSize(targetSize int, snaplen int, pageSize int) (frameSize int, blockSize int, numBlocks int, err error) {

	snaplen = 65535

	if snaplen < pageSize {
		// This is probably not quite right
		frameSize = pageSize / (pageSize / snaplen)
	} else {
		frameSize = (snaplen/pageSize + 1) * pageSize
	}

	// 128 is the default from the gopacket library so just use that
	// ???
	blockSize = frameSize * 128
	numBlocks = targetSize / blockSize

	if numBlocks == 0 {
		return 0, 0, 0, fmt.Errorf("Interface buffersize is too small")
	}

	return frameSize, blockSize, numBlocks, nil
}

func newAfpacketHandle(device string, snaplen int, blockSize int, num_blocks int, useVLAN bool, timeout time.Duration) (h *afpacket.TPacket, err error) {

	opts := []interface{}{
		afpacket.OptFrameSize(snaplen),
		afpacket.OptBlockSize(blockSize),
		afpacket.OptNumBlocks(num_blocks),
		afpacket.OptAddVLANHeader(useVLAN),
		afpacket.OptPollTimeout(CaptureTimeout),
		afpacket.SocketRaw,
		afpacket.TPacketVersion3,
	}

	if !strings.EqualFold(device, "any") {
		opts = append(opts, afpacket.OptInterface(device))
	}

	return afpacket.NewTPacket(opts...)
}
