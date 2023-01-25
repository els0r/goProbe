//go:build !linux || force_pcap
// +build !linux force_pcap

package capture

import "github.com/google/gopacket/pcap"

var errCaptureTimeout = pcap.NextErrorTimeoutExpired

func newSource() Source {
	return &PcapSource{}
}
