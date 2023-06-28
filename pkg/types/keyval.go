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
	key.PutAllV4(sip, dip, dport, proto)

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
	key.PutAllV6(sip, dip, dport, proto)

	return
}

// NewKey creates and populates a new key, determining IPv4 / IPv6 automatically from
// the length of the sip / dip inputs
func NewKey(sip, dip, dport []byte, proto byte) (key Key) {
	if len(sip) != len(dip) {
		panic("unexpected IPv4 / IPv6 mixture")
	}

	if len(sip) == IPv4Width {
		key = NewV4Key(sip, dip, dport, proto)
	} else {
		key = NewV6Key(sip, dip, dport, proto)
	}

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
	panic(fmt.Sprintf("key `%v` is neither ipv4 nor ipv6", []byte(k)))
}

// Len returns the length of the key (e.g. to determine the IP version)
func (k Key) Len() int {
	return len(k)
}

// PutAllV4 stores all elements into an existing key (assuming it is an IPv4 key)
func (k Key) PutAllV4(sip, dip, dport []byte, proto byte) {
	k.PutSip(sip)
	k.PutDipV4(dip)
	k.PutDportV4(dport)
	k.PutProtoV4(proto)
}

// PutAllV6 stores all elements into an existing key (assuming it is an IPv6 key)
func (k Key) PutAllV6(sip, dip, dport []byte, proto byte) {
	k.PutSip(sip)
	k.PutDipV6(dip)
	k.PutDportV6(dport)
	k.PutProtoV6(proto)
}

// PutDport stores a destination port in the key
func (k Key) PutDport(dport []byte) {
	k.PutDportV(dport, k.IsIPv4())
}

// PutProto stores a protocol in the key
func (k Key) PutProto(proto byte) {
	k.PutProtoV(proto, k.IsIPv4())
}

// PutSip stores a source IP in the key
func (k Key) PutSip(sip []byte) {
	copy(k[sipPos:], sip)
}

// PutDip stores a destination IP in the key
func (k Key) PutDip(dip []byte) {
	k.PutDipV(dip, k.IsIPv4())
}

// PutDipV stores a destination IP in the key (depending on the IP protocol version)
func (k Key) PutDipV(dip []byte, isIPv4 bool) {
	if isIPv4 {
		k.PutDipV4(dip)
	} else {
		k.PutDipV6(dip)
	}
}

// PutDportV stores a destination port in the key (depending on the IP protocol version)
func (k Key) PutDportV(dport []byte, isIPv4 bool) {
	if isIPv4 {
		k.PutDportV4(dport)
	} else {
		k.PutDportV6(dport)
	}
}

// PutProtoV stores a protocol in the key (depending on the IP protocol version)
func (k Key) PutProtoV(proto byte, isIPv4 bool) {
	if isIPv4 {
		k.PutProtoV4(proto)
	} else {
		k.PutProtoV6(proto)
	}
}

// PutDportV4 stores a destination port in the key (assuming it is an IPv4 key)
func (k Key) PutDportV4(dport []byte) {
	copy(k[dportPosIPv4:dportPosIPv4+DPortWidth], dport)
}

// PutProtoV4 stores a protocol in the key (assuming it is an IPv4 key)
func (k Key) PutProtoV4(proto byte) {
	k[protoPosIPv4] = proto
}

// PutDipV4 stores a destination IP in the key (assuming it is an IPv4 key)
func (k Key) PutDipV4(dip []byte) {
	copy(k[dipPosIPv4:dipPosIPv4+IPv4Width], dip)
}

// PutDportV6 stores a destination port in the key (assuming it is an IPv6 key)
func (k Key) PutDportV6(dport []byte) {
	copy(k[dportPosIPv6:dportPosIPv6+DPortWidth], dport)
}

// PutProtoV6 stores a protocol in the key (assuming it is an IPv6 key)
func (k Key) PutProtoV6(proto byte) {
	k[protoPosIPv6] = proto
}

// PutDipV6 stores a destination IP in the key (assuming it is an IPv6 key)
func (k Key) PutDipV6(dip []byte) {
	copy(k[dipPosIPv6:dipPosIPv6+IPv6Width], dip)
}

// GetDport retrieves the destination port from the key
func (k Key) GetDport() []byte {
	if k.IsIPv4() {
		return k[dportPosIPv4 : dportPosIPv4+DPortWidth]
	}
	return k[dportPosIPv6 : dportPosIPv6+DPortWidth]
}

// GetProto retrieves the protocol from the key
func (k Key) GetProto() byte {
	if k.IsIPv4() {
		return k[protoPosIPv4]
	}
	return k[protoPosIPv6]
}

// GetSip retrieves the source IP from the key
func (k Key) GetSip() []byte {
	if k.IsIPv4() {
		return k[sipPos : sipPos+IPv4Width]
	}
	return k[sipPos : sipPos+IPv6Width]
}

// GetDip retrieves the destination IP from the key
func (k Key) GetDip() []byte {
	if k.IsIPv4() {
		return k[dipPosIPv4 : dipPosIPv4+IPv4Width]
	}
	return k[dipPosIPv6 : dipPosIPv6+IPv6Width]
}

// Extend extends a "normal" key by wrapping it in an "ExtendedKey" and appending any
// additional parameters to it
func (k Key) Extend(ts int64) (e ExtendedKey) {

	// If no timestamp was provided, just convert the Key to an ExtendedKey
	if ts <= 0 {
		return ExtendedKey(k)
	}

	// Allocate a copy of sufficient size
	requiredLen := len(k) + TimestampWidth
	e = make(ExtendedKey, requiredLen)

	// Copy basic key into the new, extended one
	pos := copy(e, k)

	// Encode the timestamp
	binary.BigEndian.PutUint64(e[pos:pos+8], uint64(ts))

	return
}

// ExtendEmpty extends a "normal" key by wrapping it in an "ExtendedKey", filling
// no additional information
func (k Key) ExtendEmpty() (e ExtendedKey) {
	return k.Extend(0)
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
		return Key(e[:KeyWidthIPv4])
	}
	return Key(e[:KeyWidthIPv6])
}

// IsIPv4 returns if the key represents an IPv4 packet / flow
func (e ExtendedKey) IsIPv4() bool {
	if len(e) == KeyWidthIPv4 || len(e) == KeyWidthIPv4+TimestampWidth {
		return true
	}
	if len(e) == KeyWidthIPv6 || len(e) == KeyWidthIPv6+TimestampWidth {
		return false
	}
	panic(fmt.Sprintf("extended key `%v` is neither ipv4 nor ipv6", []byte(e)))
}

// PutSip stores a source IP in the key
func (e ExtendedKey) PutSip(sip []byte) {
	copy(e[sipPos:], sip)
}

// PutDip stores a destination IP in the key
func (e ExtendedKey) PutDip(dip []byte) {
	e.PutDipV(dip, e.IsIPv4())
}

// PutDport stores a destination port in the key
func (e ExtendedKey) PutDport(dport []byte) {
	e.PutDportV(dport, e.IsIPv4())
}

// PutProto stores a protocol in the key
func (e ExtendedKey) PutProto(proto byte) {
	e.PutProtoV(proto, e.IsIPv4())
}

// PutDipV stores a destination IP in the key (depending on the IP protocol version)
func (e ExtendedKey) PutDipV(dip []byte, isIPv4 bool) {
	if isIPv4 {
		e.PutDipV4(dip)
	} else {
		e.PutDipV6(dip)
	}
}

// PutDportV stores a destination port in the key (depending on the IP protocol version)
func (e ExtendedKey) PutDportV(dport []byte, isIPv4 bool) {
	if isIPv4 {
		e.PutDportV4(dport)
	} else {
		e.PutDportV6(dport)
	}
}

// PutProtoV stores a protocol in the key (depending on the IP protocol version)
func (e ExtendedKey) PutProtoV(proto byte, isIPv4 bool) {
	if isIPv4 {
		e.PutProtoV4(proto)
	} else {
		e.PutProtoV6(proto)
	}
}

// PutDipV4 stores a destination IP in the key (assuming it is an IPv4 key)
func (e ExtendedKey) PutDipV4(dip []byte) {
	copy(e[dipPosIPv4:dipPosIPv4+IPv4Width], dip)
}

// PutDportV4 stores a destination port in the key (assuming it is an IPv4 key)
func (e ExtendedKey) PutDportV4(dport []byte) {
	copy(e[dportPosIPv4:dportPosIPv4+DPortWidth], dport)
}

// PutProtoV4 stores a protocol in the key (assuming it is an IPv4 key)
func (e ExtendedKey) PutProtoV4(proto byte) {
	e[protoPosIPv4] = proto
}

// PutDipV6 stores a destination IP in the key (assuming it is an IPv6 key)
func (e ExtendedKey) PutDipV6(dip []byte) {
	copy(e[dipPosIPv6:dipPosIPv6+IPv6Width], dip)
}

// PutDportV6 stores a destination port in the key (assuming it is an IPv6 key)
func (e ExtendedKey) PutDportV6(dport []byte) {
	copy(e[dportPosIPv6:dportPosIPv6+DPortWidth], dport)
}

// PutProtoV6 stores a protocol in the key (assuming it is an IPv6 key)
func (e ExtendedKey) PutProtoV6(proto byte) {
	e[protoPosIPv6] = proto
}

// GetDport retrieves the destination port from the key
func (e ExtendedKey) GetDport() []byte {
	if e.IsIPv4() {
		return e[dportPosIPv4 : dportPosIPv4+DPortWidth]
	}
	return e[dportPosIPv6 : dportPosIPv6+DPortWidth]
}

// GetProto retrieves the protocol from the key
func (e ExtendedKey) GetProto() byte {
	if e.IsIPv4() {
		return e[protoPosIPv4]
	}
	return e[protoPosIPv6]
}

// GetSip retrieves the source IP from the key
func (e ExtendedKey) GetSip() []byte {
	if e.IsIPv4() {
		return e[sipPos : sipPos+IPv4Width]
	}
	return e[sipPos : sipPos+IPv6Width]
}

// GetDip retrieves the destination IP from the key
func (e ExtendedKey) GetDip() []byte {
	if e.IsIPv4() {
		return e[dipPosIPv4 : dipPosIPv4+IPv4Width]
	}
	return e[dipPosIPv6 : dipPosIPv6+IPv6Width]
}

// AttrTime retrieves the time extension (indicating its presence via the second result parameter)
func (e ExtendedKey) AttrTime() (int64, bool) {
	if len(e) == KeyWidthIPv4 || len(e) == KeyWidthIPv6 {
		return 0, false
	}

	return int64(binary.BigEndian.Uint64(e[len(e)-8:])), true
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
