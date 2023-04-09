package capture

import (
	"net"
	"time"

	"github.com/fako1024/slimcap/capture"
	"github.com/fako1024/slimcap/event"
	"github.com/fako1024/slimcap/link"
)

type mockCapture struct {
	nPkts, maxPkts int
	block          chan event.EvtData
}

func newMockCaptureSource(maxPkts int) *mockCapture {
	return &mockCapture{
		maxPkts: maxPkts,
		block:   make(chan event.EvtData, 1),
	}
}

// NewPacket creates an empty "buffer" package to be used as destination for the NextPacketInto()
// method. It ensures that a valid packet of appropriate structure / length is created
func (c *mockCapture) NewPacket() capture.Packet {
	panic("not implemented") // TODO: Implement
}
func (c *mockCapture) NextIPPacket(pBuf capture.IPLayer) (capture.IPLayer, capture.PacketType, uint32, error) {
	panic("not implemented") // TODO: Implement
}

// NextPacket receives the next packet from the wire and returns it. The operation is blocking. In
// case a non-nil "buffer" Packet is provided it will be populated with the data (and returned). The
// buffer packet can be reused. Otherwise a new Packet of the Source-specific type is allocated.
func (c *mockCapture) NextPacket(pBuf capture.Packet) (capture.Packet, error) {
	if c.maxPkts >= 0 && c.nPkts >= c.maxPkts {
		// Simlutate blocking operation (but ensure we cannot enter a deadlock by timing out)
		// Also respond to a Close() event
		select {
		case evt := <-c.block:
			if evt == event.SignalUnblock {
				return nil, capture.ErrCaptureUnblock
			}
			if evt == event.SignalStop {
				return nil, capture.ErrCaptureStopped
			}

		case <-time.After(5 * time.Second):
			return nil, capture.ErrCaptureStopped
		}
	}

	select {
	case evt := <-c.block:
		if evt == event.SignalStop {
			return nil, capture.ErrCaptureStopped
		}
	default:
	}

	capture.NewIPPacket(pBuf, []byte{0x40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x6, 0x0, 0x0, 0xa, 0x0, 0x0, 0x1, 0xa, 0x0, 0x0, 0x2, 0x92, 0x6d, 0x44, 0x5c, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}, 0, 128)

	c.nPkts++
	return pBuf, nil
}

// NextIPPacketFn executes the provided function on the next packet received on the wire and only
// return the ring buffer block to the kernel upon completion of the function. If possible, the
// operation should provide a zero-copy way of interaction with the payload / metadata.
func (c *mockCapture) NextPacketFn(_ func(payload []byte, totalLen uint32, pktType capture.PacketType, ipLayerOffset byte) error) error {
	panic("not implemented") // TODO: Implement
}

// Stats returns (and clears) the packet counters of the underlying socket
func (c *mockCapture) Stats() (capture.Stats, error) {
	return capture.Stats{}, nil
}

// Link returns the underlying link
func (c *mockCapture) Link() *link.Link {
	return &link.Link{
		LinkType:  65534,
		Interface: &net.Interface{},
	}
}

// Unblock ensures that a potentially ongoing blocking PPOLL is released (returning an ErrCaptureUnblock)
func (c *mockCapture) Unblock() error {
	select {
	case c.block <- event.SignalUnblock:
	default:
	}

	return nil
}

// Close stops / closes the capture source
func (c *mockCapture) Close() error {
	select {
	case c.block <- event.SignalStop:
	default:
	}

	return nil
}

// Free releases any pending resources from the capture source (must be called after Close())
func (c *mockCapture) Free() error {
	return nil
}
