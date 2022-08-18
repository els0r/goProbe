//go:build !linux || force_pcap
// +build !linux force_pcap

package capture

import "github.com/fako1024/gopacket/pcap"

var errCaptureTimeout = pcap.NextErrorTimeoutExpired

func newSource() Source {
	return &PcapSource{}
}
