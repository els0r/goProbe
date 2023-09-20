package hashmap

import "github.com/els0r/goProbe/pkg/types"

// Type definitions for easy modification
type (

	// Key defines the Key type of the map
	Key = []byte

	// Val defines the value / valent type of the map
	Val = types.Counters
)

// KeyVal denotes a key / value pair
type KeyVal struct {
	Key Key
	Val Val
}

// KeyVals denotes a list / slice of key / value pairs
type KeyVals []KeyVal

// ValFilter denotes a generic value filter (returning true if a certain filter
// condition is met)
type ValFilter func(Val) bool

// New instantiates a new Map (a size hint can be provided)
func New(n ...int) *Map {
	if len(n) == 0 || n[0] == 0 {
		return NewHint(0)
	}
	m := NewHint(n[0])
	return m
}

// AggFlowMap stores all flows where the source port from the FlowLog has been aggregated
// Just a convenient alias for the map type itself
type AggFlowMap struct {
	PrimaryMap   *Map
	SecondaryMap *Map
}

// NewAggFlowMap instantiates a new NewAggFlowMap with an underlying
// hashmap for both IPv4 and IPv6 entries
func NewAggFlowMap(n ...int) *AggFlowMap {
	return &AggFlowMap{
		PrimaryMap:   New(n...),
		SecondaryMap: New(n...),
	}
}

// NilAggFlowMapWithMetadata denotes an empty / "nil" AggFlowMapWithMetadata
var NilAggFlowMapWithMetadata = AggFlowMapWithMetadata{}

// AggFlowMapWithMetadata provides a wrapper around the map with ancillary data
type AggFlowMapWithMetadata struct {
	*AggFlowMap

	Interface string `json:"iface"`
}

// NamedAggFlowMapWithMetadata provides wrapper around a map of AggFlowMapWithMetadata
// instances (e.g. interface -> AggFlowMapWithMetadata associations)
type NamedAggFlowMapWithMetadata map[string]*AggFlowMapWithMetadata

// NewNamedAggFlowMapWithMetadata instantiates a new NewNamedAggFlowMapWithMetadata based
// on a list of names, initializing an instance of AggFlowMapWithMetadata per element
func NewNamedAggFlowMapWithMetadata(names []string) (m NamedAggFlowMapWithMetadata) {
	m = make(NamedAggFlowMapWithMetadata)
	for _, name := range names {
		obj := NewAggFlowMapWithMetadata()
		m[name] = &obj
	}
	return
}

// Len returns the number of entries in all maps
func (n NamedAggFlowMapWithMetadata) Len() (l int) {
	for _, v := range n {
		l += v.Len()
	}
	return
}

// Clear frees as many resources as possible by making them eligible for GC
func (n NamedAggFlowMapWithMetadata) Clear() {
	for k, v := range n {
		v.Clear()
		delete(n, k)
	}
}

// ClearFast nils all main resources, making them eligible for GC (but
// probably not as effectively as Clear())
func (n NamedAggFlowMapWithMetadata) ClearFast() {
	for _, v := range n {
		v.ClearFast()
	}
}

// NewAggFlowMapWithMetadata instantiates a new AggFlowMapWithMetadata with an underlying
// hashmap for both IPv4 and IPv6 entries
func NewAggFlowMapWithMetadata(n ...int) AggFlowMapWithMetadata {
	return AggFlowMapWithMetadata{
		AggFlowMap: &AggFlowMap{
			PrimaryMap:   New(n...),
			SecondaryMap: New(n...),
		},
	}
}

// IsNil returns if an AggFlowMapWithMetadata is nil (used e.g. in cases of error)
func (a AggFlowMapWithMetadata) IsNil() bool {
	if a.AggFlowMap == nil {
		return true
	}
	return a.AggFlowMap.IsNil()
}

// IsNil returns if an AggFlowMap is nil (used e.g. in cases of error)
func (a AggFlowMap) IsNil() bool {
	return a.PrimaryMap == nil && a.SecondaryMap == nil
}

// Len returns the number of valents in the map
func (a AggFlowMap) Len() int {
	return a.PrimaryMap.count + a.SecondaryMap.count
}

// Iter provides a map Iter to allow traversal of both underlying maps (IPv4 and IPv6)
func (a AggFlowMap) Iter(opts ...MetaIterOption) *MetaIter {
	iter := &MetaIter{
		Iter:          a.PrimaryMap.Iter(),
		secondaryIter: a.SecondaryMap.Iter(),
	}

	// Set functional options, if any
	for _, opt := range opts {
		opt(iter)
	}
	return iter
}

// SetOrUpdate either creates a new entry based on the provided values or
// updates any existing valent (if exists). This way may be very specific, but
// it avoids intermediate allocation of a value type valent in case of an update
func (a AggFlowMap) SetOrUpdate(key Key, isIPv4 bool, eA, eB, eC, eD uint64) {
	if isIPv4 {
		a.PrimaryMap.SetOrUpdate(key, eA, eB, eC, eD)
	} else {
		a.SecondaryMap.SetOrUpdate(key, eA, eB, eC, eD)
	}
}

// Merge allows to incorporate the content of a map b into an existing map a
func (a AggFlowMap) Merge(b AggFlowMap) {
	a.PrimaryMap.Merge(b.PrimaryMap)
	a.SecondaryMap.Merge(b.SecondaryMap)
}

// Merge allows to incorporate the content of a map b into an existing map a
func (a AggFlowMapWithMetadata) Merge(b AggFlowMapWithMetadata) {
	a.PrimaryMap.Merge(b.PrimaryMap)
	a.SecondaryMap.Merge(b.SecondaryMap)
}

// Clear frees as many resources as possible by making them eligible for GC
func (a AggFlowMap) Clear() {
	a.PrimaryMap.Clear()
	a.SecondaryMap.Clear()
}

// ClearFast nils all main resources, making them eligible for GC (but
// probably not as effectively as Clear())
func (a AggFlowMap) ClearFast() {
	a.PrimaryMap.ClearFast()
	a.SecondaryMap.ClearFast()
}
