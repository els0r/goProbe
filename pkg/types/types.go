package types

import (
	"net/netip"
)

type Status string

const (
	StatusError Status = "error"
	StatusEmpty Status = "empty"
	StatusOK    Status = "ok"
)

const DefaultTimeOutputFormat = "2006-01-02 15:04:05"

type Width = int

const (
	IPv6Width  Width = 16
	IPv4Width  Width = 4
	DPortWidth Width = 2
	ProtoWidth Width = 1

	TimestampWidth Width = 8
)

const (
	KeyWidthIPv6 = 2*IPv6Width + DPortWidth + ProtoWidth
	KeyWidthIPv4 = 2*IPv4Width + DPortWidth + ProtoWidth

	SipPos     = 0
	DipPosIPv4 = IPv4Width
	DipPosIPv6 = IPv6Width

	DPortPosIPv4 = IPv4Width + IPv4Width
	DPortPosIPv6 = IPv6Width + IPv6Width
	ProtoPosIPv4 = DPortPosIPv4 + DPortWidth
	ProtoPosIPv6 = DPortPosIPv6 + DPortWidth
)

// RawIPToAddr converts an ip byte slice to an actual netip.Addr
func RawIPToAddr(ip []byte) netip.Addr {
	netIP, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}
	}
	return netIP
}

// RawIPToString converts an ip byte slice to string
func RawIPToString(ip []byte) string {
	return RawIPToAddr(ip).String()
}
