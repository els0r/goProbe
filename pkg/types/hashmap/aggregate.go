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
type AggFlowMap = Map

// NilAggFlowMapWithMetadata denotes an empty / "nil" AggFlowMapWithMetadata
var NilAggFlowMapWithMetadata = AggFlowMapWithMetadata{}

// AggFlowMapWithMetadata provides a wrapper around the map with ancillary data
type AggFlowMapWithMetadata struct {
	V4Map, V6Map *AggFlowMap

	HostID    uint   `json:"host_id"`
	Hostname  string `json:"host"`
	Interface string `json:"iface"`
}

type NamedAggFlowMapWithMetadata map[string]*AggFlowMapWithMetadata

func NewNamedAggFlowMapWithMetadata(names []string) (m NamedAggFlowMapWithMetadata) {
	m = make(NamedAggFlowMapWithMetadata)
	for _, name := range names {
		obj := NewAggFlowMapWithMetadata()
		m[name] = &obj
	}
	return
}

// Len returns the number of valents in the map
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
		// delete(n, k)
	}
}

// NewAggFlowMapWithMetadata instantiates a new AggFlowMapWithMetadata with an underlying
// hashmap
func NewAggFlowMapWithMetadata(n ...int) AggFlowMapWithMetadata {
	return AggFlowMapWithMetadata{
		V4Map: New(n...),
		V6Map: New(n...),
	}
}

// IsNil returns if an AggFlowMapWithMetadata is nil (used e.g. in cases of error)
func (a AggFlowMapWithMetadata) IsNil() bool {
	return a.V4Map == nil && a.V6Map == nil
}

// Len returns the number of valents in the map
func (a AggFlowMapWithMetadata) Len() int {
	return a.V4Map.count + a.V6Map.count
}

func (a AggFlowMapWithMetadata) Iter() *MetaIter {
	return &MetaIter{
		Iter:   a.V4Map.Iter(),
		v6Iter: a.V6Map.Iter(),
	}
}

func (a AggFlowMapWithMetadata) Merge(b AggFlowMapWithMetadata, totals *Val) {
	a.V4Map.Merge(b.V4Map, totals)
	a.V6Map.Merge(b.V6Map, totals)
}

// Clear frees as many resources as possible by making them eligible for GC
func (a AggFlowMapWithMetadata) Clear() {
	a.V4Map.Clear()
	a.V6Map.Clear()
}

// ClearFast nils all main resources, making them eligible for GC (but
// probably not as effectively as Clear())
func (a AggFlowMapWithMetadata) ClearFast() {
	a.V4Map.ClearFast()
	a.V6Map.ClearFast()
}
