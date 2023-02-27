package hashmap

import (
	"bytes"
	"sort"

	"github.com/els0r/goProbe/pkg/types"
)

// Item denotes a flat key / value pair
type Item struct {
	types.Key
	Val
}

// List denotes a list of key / value pairs
type List []Item

// Flatten converts a flow map to a flat table / list
func (m *Map) Flatten() (v4List List, v6List List) {
	v4List, v6List = make(List, 0), make(List, 0)

	for i := m.Iter(); i.Next(); {
		if k := types.Key(i.Key()); k.IsIPv4() {
			v4List = append(v4List, Item{k, i.Val()})
		} else {
			v6List = append(v6List, Item{k, i.Val()})
		}
	}

	return
}

// Sort orders relevant flow columns so that they become more compressible
func (l List) Sort() List {
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
