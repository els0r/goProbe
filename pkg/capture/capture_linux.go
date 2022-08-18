//go:build linux && !force_pcap
// +build linux,!force_pcap

package capture

import "github.com/fako1024/gopacket/afpacket"

var errCaptureTimeout = afpacket.ErrTimeout

func newSource() Source {
	return &AFPacketSource{}
}
