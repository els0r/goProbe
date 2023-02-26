/////////////////////////////////////////////////////////////////////////////////
//
// keyval.go
//
// Flow map primitives and their utility functions
//
// Written by Lennart Elsen lel@open.ch
//
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package types

import (
	"encoding/binary"
	"fmt"

	"github.com/els0r/goProbe/pkg/goDB/protocols"
)

// Key stores the 5-tuple which defines a goProbe flow
type Key []byte

// NewEmptyV4Key creates / allocates an emty key for IPV4
func NewEmptyV4Key() Key {
	return make(Key, KeyWidthIPv4)
}

// NewV4KeyStatic creates / allocates an emty key for IPV4 (parsing IPs from arrays)
func NewV4KeyStatic(sip, dip [4]byte, dport []byte, proto byte) Key {
	return NewV4Key(sip[:], dip[:], dport[:], proto)
}

// NewV4Key creates and populates a new key for IPv4
func NewV4Key(sip, dip, dport []byte, proto byte) (key Key) {
	key = NewEmptyV4Key()

	key.PutDport(dport)
	key.PutProto(proto)
	key.PutSip(sip)
	key.PutDip(dip)

	return
}

// NewEmptyV6Key creates / allocates an emty key for IPV6
func NewEmptyV6Key() Key {
	return make(Key, KeyWidthIPv6)
}

// NewV6KeyStatic creates / allocates an emty key for IPV6 (parsing IPs from arrays)
func NewV6KeyStatic(sip, dip [16]byte, dport []byte, proto byte) Key {
	return NewV6Key(sip[:], dip[:], dport[:], proto)
}

// NewV6Key creates and populates a new key for IPv6
func NewV6Key(sip, dip, dport []byte, proto byte) (key Key) {
	key = NewEmptyV6Key()

	key.PutDport(dport)
	key.PutProto(proto)
	key.PutSip(sip)
	key.PutDip(dip)

	return
}

// NewKey creates and populates a new key, determining IPv4 / IPv6 automatically from
// the length of the sip / dip inputs
func NewKey(sip, dip, dport []byte, proto byte) (key Key) {
	if len(sip) != len(dip) {
		panic("unexpected IPv4 / IPv6 mixture")
	}

	if len(sip) == IPv4Width {
		key = NewEmptyV4Key()
	} else {
		key = NewEmptyV6Key()
	}

	key.PutDport(dport)
	key.PutProto(proto)
	key.PutSip(sip)
	key.PutDip(dip)

	return
}

// Clone provides a copy of the key
func (k Key) Clone() Key {
	cp := make(Key, len(k))
	copy(cp, k)
	return cp
}

// IsIPv4 returns if a key represents an IPv4 flow (based on its length)
func (k Key) IsIPv4() bool {
	if len(k) == KeyWidthIPv4 {
		return true
	}
	if len(k) == KeyWidthIPv6 {
		return false
	}
	panic("key is neither ipv4 nor ipv6")
}

// Len returns the length of the key (e.g. to determine the IP version)
func (k Key) Len() int {
	return len(k)
}

// PutDport stores a destination port in the key
func (k Key) PutDport(dport []byte) {
	copy(k[DPortPos:DPortPos+DPortWidth], dport[:])
}

// PutProto stores a protocol in the key
func (k Key) PutProto(proto byte) {
	copy(k[ProtoPos:ProtoPos+ProtoWidth], []byte{proto})
}

// PutSip stores a source IP in the key
func (k Key) PutSip(sip []byte) {
	copy(k[SipPos:], sip)
}

// PutDip stores a destination IP in the key
func (k Key) PutDip(dip []byte) {
	if len(dip) == IPv4Width {
		copy(k[DipPosIPv4:], dip)
	} else {
		copy(k[DipPosIPv6:], dip)
	}
}

// GetDport retrieves the destination port from the key
func (k Key) GetDport() []byte {
	return k[DPortPos : DPortPos+DPortWidth]
}

// GetProto retrieves the protocol from the key
func (k Key) GetProto() byte {
	return k[ProtoPos]
}

// GetSip retrieves the source IP from the key
func (k Key) GetSip() []byte {
	if k.IsIPv4() {
		return k[SipPos : SipPos+IPv4Width]
	}
	return k[SipPos : SipPos+IPv6Width]
}

// GetDip retrieves the destination IP from the key
func (k Key) GetDip() []byte {
	if k.IsIPv4() {
		return k[DipPosIPv4 : DipPosIPv4+IPv4Width]
	}
	return k[DipPosIPv6 : DipPosIPv6+IPv6Width]
}

// Extend extends a "normal" key by wrapping it in an "ExtendedKey" and appending any
// additional parameters to it
func (k Key) Extend(ts int64, iface string) (e ExtendedKey) {

	requiredLen := len(k) + 1 // Add one byte to store bitmask for key type / content
	if ts > 0 {
		requiredLen += 8
	}
	if iface != "" {
		requiredLen += len(iface)
	}
	e = make(ExtendedKey, requiredLen)

	// Copy basic key into the new, extended one and flag it as IPv4 or IPv6
	pos := 1 + copy(e[1:1+len(k)], k)
	if !k.IsIPv4() {
		e[0] |= (1 << 0)
	}

	// If provided, encode the timestamp and add its flag
	if ts > 0 {
		binary.BigEndian.PutUint64(e[pos:pos+8], uint64(ts))
		e[0] |= (1 << 1)
		pos += 8
	}

	// If provided, append the interface name and add its flag
	if iface != "" {
		copy(e[pos:], iface)
		e[0] |= (1 << 2)
	}

	return
}

// ExtendEmpty extends a "normal" key by wrapping it in an "ExtendedKey", filling
// no additional information
func (k Key) ExtendEmpty() (e ExtendedKey) {
	return k.Extend(0, "")
}

// ExtendedKey is a Key with supplemental information
type ExtendedKey []byte

// Clone provides a copy of the extended key
func (e ExtendedKey) Clone() ExtendedKey {
	cp := make(ExtendedKey, len(e))
	copy(cp, e)
	return cp
}

// Key retrieves the basic key within the extended key to allow for
// more precise access without having to always use the (longer) ExtendedKey
func (e ExtendedKey) Key() Key {
	if e.IsIPv4() {
		return Key(e[1 : 1+KeyWidthIPv4])
	}
	return Key(e[1 : 1+KeyWidthIPv6])
}

// IsIPv4 returns if the key represents an IPv4 packet / flow
func (e ExtendedKey) IsIPv4() bool {
	return e[0]&(1<<0) <= 0
}

// PutSip stores a source IP in the key
func (e ExtendedKey) PutSip(sip []byte) {
	copy(e[1+SipPos:], sip)
}

// PutSipV4 stores an IPv4 source IP in the key
func (e ExtendedKey) PutSipV4(sip []byte) {
	copy(e[1+SipPos:1+SipPos+IPv4Width], sip)
}

// PutSipV6 stores an IPv6 source IP in the key
func (e ExtendedKey) PutSipV6(sip []byte) {
	copy(e[1+SipPos:1+SipPos+IPv6Width], sip)
}

// PutDip stores a destination IP in the key
func (e ExtendedKey) PutDip(dip []byte) {
	if len(dip) == IPv4Width {
		copy(e[1+DipPosIPv4:], dip)
	} else {
		copy(e[1+DipPosIPv6:], dip)
	}
}

// PutDipV4 stores an IPv4 destination IP in the key
func (e ExtendedKey) PutDipV4(dip []byte) {
	copy(e[1+DipPosIPv4:1+DipPosIPv4+IPv4Width], dip)
}

// PutDipV6 stores an IPv6 destination IP in the key
func (e ExtendedKey) PutDipV6(dip []byte) {
	copy(e[1+DipPosIPv6:1+DipPosIPv6+IPv6Width], dip)
}

// PutDport stores a destination port in the key
func (e ExtendedKey) PutDport(dport []byte) {
	copy(e[1+DPortPos:1+DPortPos+DPortWidth], dport[:])
}

// PutProto stores a protocol in the key
func (e ExtendedKey) PutProto(proto byte) {
	copy(e[1+ProtoPos:1+ProtoPos+ProtoWidth], []byte{proto})
}

// AttrTime retrieves the time extension (indicating its presence via the second result parameter)
func (e ExtendedKey) AttrTime() (int64, bool) {
	if e[0]&(1<<1) > 0 {
		if e.IsIPv4() {
			return int64(binary.BigEndian.Uint64(e[1+KeyWidthIPv4 : 9+KeyWidthIPv4])), true
		}
		return int64(binary.BigEndian.Uint64(e[1+KeyWidthIPv6 : 9+KeyWidthIPv6])), true
	}
	return 0, false
}

// AttrIface retrieves the interface name extension (indicating its presence via the second result parameter)
func (e ExtendedKey) AttrIface() (string, bool) {
	if e[0]&(1<<2) > 0 {
		pos := 1
		if e.IsIPv4() {
			pos += KeyWidthIPv4
		} else {
			pos += KeyWidthIPv6
		}
		if e[0]&(1<<1) > 0 {
			pos += 8
		}
		return string(e[pos:]), true
	}
	return "", false
}

// String prints the key as a comma separated attribute list
func (k Key) String() string {
	return fmt.Sprintf("%s,%s,%d,%s",
		RawIPToString(k.GetSip()),
		RawIPToString(k.GetDip()),
		int(PortToUint16(k.GetDport())),
		protocols.GetIPProto(int(k.GetProto())),
	)
}

// Counters stores the goProbe flow counters (and, where required, some extensions)
type Counters struct {
	BytesRcvd   uint64 `json:"br,omitempty"`
	BytesSent   uint64 `json:"bs,omitempty"`
	PacketsRcvd uint64 `json:"pr,omitempty"`
	PacketsSent uint64 `json:"ps,omitempty"`
}

// String prints the flow counters
func (c Counters) String() string {
	return fmt.Sprintf("bytes: received=%d sent=%d; packets: received=%d sent=%d",
		c.BytesRcvd,
		c.BytesSent,
		c.PacketsRcvd,
		c.PacketsSent,
	)
}

// SumPackets sums the packet received and sent directions
func (c Counters) SumPackets() uint64 {
	return c.PacketsRcvd + c.PacketsSent
}

// SumBytes sums the bytes received and sent directions
func (c Counters) SumBytes() uint64 {
	return c.BytesRcvd + c.BytesSent
}

// Add adds the values from a different counter and returns the result
func (c Counters) Add(c2 Counters) Counters {
	c.BytesRcvd += c2.BytesRcvd
	c.BytesSent += c2.BytesSent
	c.PacketsRcvd += c2.PacketsRcvd
	c.PacketsSent += c2.PacketsSent
	return c
}

// Sub subtracts the values from a different counter and returns the result
func (c Counters) Sub(c2 Counters) Counters {
	c.BytesRcvd -= c2.BytesRcvd
	c.BytesSent -= c2.BytesSent
	c.PacketsRcvd -= c2.PacketsRcvd
	c.PacketsSent -= c2.PacketsSent
	return c
}
