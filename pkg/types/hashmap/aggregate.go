package hashmap

import "github.com/els0r/goProbe/pkg/types"

// Type definitions for easy modification
type (

	// K defines the Key type of the map
	Key = []byte

	// E defines the value / valent type of the map
	Val = types.Counters
)

// KeyVal denotes a key / value pair
type KeyVal struct {
	Key Key
	Val Val
}

// KeyVals denotes a list / slice of key / value pairs
type KeyVals []KeyVal

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
	V4Map *Map
	V6Map *Map
}

// NewAggFlowMap instantiates a new NewAggFlowMap with an underlying
// hashmap for both IPv4 and IPv6 entries
func NewAggFlowMap(n ...int) *AggFlowMap {
	return &AggFlowMap{
		V4Map: New(n...),
		V6Map: New(n...),
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

// Flatten converts a flow map to a flat table / list
func (n *NamedAggFlowMapWithMetadata) Flatten() (list AggregateList) {
	if n == nil {
		return
	}

	list = make(AggregateList, n.Len())
	count := 0
	for iface, m := range *n {
		for it := m.Iter(); it.Next(); {
			list[count] = AggregateItem{
				Item: Item{ExtendedKey: it.Key(), Val: it.Val()},

				// TODO: Avoid this duplication and wrap in metadata (would make sorting by iface easier as well maybe)
				// Then we could also just use an Item / []Item instead of []AggregateItem
				Iface: iface,
			}
			count++
		}
	}

	return
}

// Len returns the number of entries in all maps
func (n *NamedAggFlowMapWithMetadata) Len() (l int) {
	if n == nil {
		return
	}
	for _, v := range *n {
		l += v.Len()
	}
	return
}

// Clear frees as many resources as possible by making them eligible for GC
func (n *NamedAggFlowMapWithMetadata) Clear() {
	if n == nil {
		return
	}
	for k, v := range *n {
		v.Clear()
		delete(*n, k)
	}
}

// ClearFast nils all main resources, making them eligible for GC (but
// probably not as effectively as Clear())
func (n *NamedAggFlowMapWithMetadata) ClearFast() {
	if n == nil {
		return
	}
	for _, v := range *n {
		v.ClearFast()
	}
}

// NewAggFlowMapWithMetadata instantiates a new AggFlowMapWithMetadata with an underlying
// hashmap for both IPv4 and IPv6 entries
func NewAggFlowMapWithMetadata(n ...int) AggFlowMapWithMetadata {
	return AggFlowMapWithMetadata{
		AggFlowMap: &AggFlowMap{
			V4Map: New(n...),
			V6Map: New(n...),
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
	return a.V4Map == nil && a.V6Map == nil
}

// Len returns the number of valents in the map
func (a AggFlowMap) Len() int {
	return a.V4Map.count + a.V6Map.count
}

// Iter provides a map Iter to allow traversal of both underlying maps (IPv4 and IPv6)
func (a AggFlowMap) Iter() *MetaIter {
	return &MetaIter{
		Iter:   a.V4Map.Iter(),
		v6Iter: a.V6Map.Iter(),
	}
}

// Iter provides a map Iter to allow traversal of both underlying maps (IPv4 and IPv6)
// SetOrUpdate either creates a new entry based on the provided values or
// updates any existing valent (if exists). This way may be very specific, but
// it avoids intermediate allocation of a value type valent in case of an update
func (a AggFlowMap) SetOrUpdate(key Key, isIPv4 bool, eA, eB, eC, eD uint64) {
	if isIPv4 {
		a.V4Map.SetOrUpdate(key, eA, eB, eC, eD)
	} else {
		a.V6Map.SetOrUpdate(key, eA, eB, eC, eD)
	}
}

// Merge allows to incorporate the content of a map b into an existing map a (providing
// additional in-place counter updates).
func (a AggFlowMap) Merge(b AggFlowMap, totals *Val) {
	a.V4Map.Merge(b.V4Map, totals)
	a.V6Map.Merge(b.V6Map, totals)
}

// Merge allows to incorporate the content of a map b into an existing map a (providing
// additional in-place counter updates).
func (a AggFlowMapWithMetadata) Merge(b AggFlowMapWithMetadata, totals *Val) {
	a.V4Map.Merge(b.V4Map, totals)
	a.V6Map.Merge(b.V6Map, totals)
}

// Clear frees as many resources as possible by making them eligible for GC
func (a AggFlowMap) Clear() {
	a.V4Map.Clear()
	a.V6Map.Clear()
}

// ClearFast nils all main resources, making them eligible for GC (but
// probably not as effectively as Clear())
func (a AggFlowMap) ClearFast() {
	a.V4Map.ClearFast()
	a.V6Map.ClearFast()
}
