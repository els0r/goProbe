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
func (a *AggFlowMap) Flatten() (primaryList List, secondaryList List) {
	if a == nil {
		return
	}

	primaryList, secondaryList = make(List, a.PrimaryMap.Len()), make(List, a.SecondaryMap.Len())

	for j, it := 0, a.PrimaryMap.Iter(); it.Next(); j++ {
		primaryList[j] = Item{it.Key(), it.Val()}
	}
	for j, it := 0, a.SecondaryMap.Iter(); it.Next(); j++ {
		secondaryList[j] = Item{it.Key(), it.Val()}
	}

	return
}

// Sort orders relevant flow columns so that they become more compressible
func (l List) Sort() List {
	sort.Slice(l, func(i, j int) bool {

		iv, jv := l[i], l[j]

		if comp := bytes.Compare(iv.GetSIP(), jv.GetSIP()); comp != 0 {
			return comp < 0
		}
		if comp := bytes.Compare(iv.GetDIP(), jv.GetDIP()); comp != 0 {
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
