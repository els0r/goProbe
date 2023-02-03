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

package goDB

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/protocols"
	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
)

// Key stores the 5-tuple which defines a goProbe flow
type Key []byte

// NewEmptyV4Key creates / allocates an emty key for IPV4
func NewEmptyV4Key() Key {
	return make(Key, types.KeyWidthIPv4)
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
	return make(Key, types.KeyWidthIPv6)
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

	if len(sip) == types.IPv4Width {
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

// IsIPv4 returns if a key represents an IPv4 flow (based on its length)
func (k Key) IsIPv4() bool {
	if len(k) == types.KeyWidthIPv4 {
		return true
	}
	if len(k) == types.KeyWidthIPv6 {
		return false
	}
	panic("key is neither ipv4 nor ipv6")
}

// PutDport stores a destination port in the key
func (k Key) PutDport(dport []byte) {
	copy(k[types.DPortPos:types.DPortPos+types.DPortWidth], dport[:])
}

// PutProto stores a protocol in the key
func (k Key) PutProto(proto byte) {
	copy(k[types.ProtoPos:types.ProtoPos+types.ProtoWidth], []byte{proto})
}

// PutSip stores a source IP in the key
func (k Key) PutSip(sip []byte) {
	copy(k[types.SipPos:], sip)
}

// PutDip stores a destination IP in the key
func (k Key) PutDip(dip []byte) {
	if len(dip) == types.IPv4Width {
		copy(k[types.DipPosIPv4:], dip)
	} else {
		copy(k[types.DipPosIPv6:], dip)
	}
}

// GetDport retrieves the destination port from the key
func (k Key) GetDport() []byte {
	return k[types.DPortPos : types.DPortPos+types.DPortWidth]
}

// GetProto retrieves the protocol from the key
func (k Key) GetProto() byte {
	return k[types.ProtoPos]
}

// GetSip retrieves the source IP from the key
func (k Key) GetSip() []byte {
	if k.IsIPv4() {
		return k[types.SipPos : types.SipPos+types.IPv4Width]
	}
	return k[types.SipPos : types.SipPos+types.IPv6Width]
}

// GetDip retrieves the destination IP from the key
func (k Key) GetDip() []byte {
	if k.IsIPv4() {
		return k[types.DipPosIPv4 : types.DipPosIPv4+types.IPv4Width]
	}
	return k[types.DipPosIPv6 : types.DipPosIPv6+types.IPv6Width]
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

// ExtendedKey is a Key with supplemental information
type ExtendedKey []byte

// Key retrieves the basic key within the extended key to allow for
// more precise access without having to always use the (longer) ExtendedKey
func (e ExtendedKey) Key() Key {
	if e.IsIPv4() {
		return Key(e[1 : 1+types.KeyWidthIPv4])
	}
	return Key(e[1 : 1+types.KeyWidthIPv6])
}

// IsIPv4 returns if the key represents an IPv4 packet / flow
func (e ExtendedKey) IsIPv4() bool {
	return e[0]&(1<<0) <= 0
}

// AttrTime retrieves the time extension (indicating its presence via the second result parameter)
func (e ExtendedKey) AttrTime() (time.Time, bool) {
	if e[0]&(1<<1) > 0 {
		if e.IsIPv4() {
			return time.Unix(int64(binary.BigEndian.Uint64(e[1+types.KeyWidthIPv4:9+types.KeyWidthIPv4])), 0), true
		}
		return time.Unix(int64(binary.BigEndian.Uint64(e[1+types.KeyWidthIPv6:9+types.KeyWidthIPv6])), 0), true
	}
	return time.Time{}, false
}

// AttrIface retrieves the interface name extension (indicating its presence via the second result parameter)
func (e ExtendedKey) AttrIface() (string, bool) {
	if e[0]&(1<<2) > 0 {
		pos := 1
		if e.IsIPv4() {
			pos += types.KeyWidthIPv4
		} else {
			pos += types.KeyWidthIPv6
		}
		if e[0]&(1<<1) > 0 {
			pos += 8
		}
		return string(e[pos:]), true
	}
	return "", false
}

// Val stores the goProbe flow counters (and, where required, some extensions)
type Val struct {
	NBytesRcvd uint64 `json:"bytes_rcvd"`
	NBytesSent uint64 `json:"bytes_sent"`
	NPktsRcvd  uint64 `json:"packets_rcvd"`
	NPktsSent  uint64 `json:"packets_sent"`

	// TODO: This will be identical for flows in any map up to the point of final aggregation
	// Maybe wrap AggFlowMap in a struct and hand over these parameters _once_ as metadata ?
	HostID   uint   `json:"host_id"`
	Hostname string `json:"host"`
}

// AggFlowMap stores all flows where the source port from the FlowLog has been aggregated
type AggFlowMap map[string]*Val

type ListItem struct {
	Key
	*Val
}
type AggFlowList []ListItem

// String prints the key as a comma separated attribute list
func (k Key) String() string {
	return fmt.Sprintf("%s,%s,%d,%s",
		types.RawIPToString(k.GetSip()),
		types.RawIPToString(k.GetDip()),
		int(types.PortToUint16(k.GetDport())),
		protocols.GetIPProto(int(k.GetProto())),
	)
}

// String prints the comma-seperated flow counters
func (v *Val) String() string {
	return fmt.Sprintf("%d,%d,%d,%d",
		v.NPktsRcvd,
		v.NPktsSent,
		v.NBytesRcvd,
		v.NBytesSent,
	)
}

// MarshalJSON implements the Marshaler interface for the whole flow map
func (a AggFlowMap) MarshalJSON() ([]byte, error) {
	var toMarshal []interface{}

	for k, v := range a {
		toMarshal = append(toMarshal,
			struct {
				Attributes string `json:"attributes"`
				Counters   *Val   `json:"counters"`
			}{k, v},
		)
	}
	return jsoniter.Marshal(toMarshal)
}

// Flatten converts a flow map to a flat table / list
func (a AggFlowMap) Flatten() (v4List AggFlowList, v6List AggFlowList) {
	v4List, v6List = make(AggFlowList, 0), make(AggFlowList, 0)
	for K, V := range a {
		if k := Key(K); k.IsIPv4() {
			v4List = append(v4List, ListItem{k, V})
		} else {
			v6List = append(v6List, ListItem{k, V})
		}
	}

	return
}

// Sort orders relevant flow columns so that they become more compressible
func (l AggFlowList) Sort() AggFlowList {
	sort.Slice(l, func(i, j int) bool {

		iv, jv := l[i], l[j]

		if comp := bytes.Compare(iv.GetSip(), jv.GetSip()); comp != 0 {
			return comp < 0
		}
		if comp := bytes.Compare(iv.GetDip(), jv.GetDip()); comp != 0 {
			return comp < 0
		}
		if comp := bytes.Compare(iv.GetDport(), jv.GetDport()); comp != 0 {
			return comp < 0
		}
		if iv.GetProto() != jv.GetProto() {
			return iv.GetProto() < jv.GetProto()
		}

		return false
	})

	return l
}
