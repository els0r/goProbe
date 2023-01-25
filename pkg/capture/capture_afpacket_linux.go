//go:build linux && !force_pcap && !force_pfring
// +build linux,!force_pcap,!force_pfring

package capture

import "github.com/fako1024/gopacket/afpacket"

var errCaptureTimeout = afpacket.ErrTimeout

func newSource() Source {
	return &AFPacketSource{}
}
