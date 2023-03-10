package hashmap

import (
	"bytes"
	"sort"
	"strings"

	"github.com/els0r/goProbe/pkg/types"
)

// Item denotes a flat key / value pair
type Item struct {
	types.ExtendedKey
	Val
}

// AggregateItem denotes a flat key / value pair, extended by metadata
type AggregateItem struct {
	Item

	Iface string
}

// List denotes a list of key / value pairs
type List []Item

// List denotes a list of key / value pairs, each extended by metadata
type AggregateList []AggregateItem

// Flatten converts a flow map to a flat table / list
func (a *AggFlowMap) Flatten() (v4List List, v6List List) {
	if a == nil {
		return
	}

	v4List, v6List = make(List, a.V4Map.Len()), make(List, a.V6Map.Len())

	for j, it := 0, a.V4Map.Iter(); it.Next(); j++ {
		v4List[j] = Item{it.Key(), it.Val()}
	}
	for j, it := 0, a.V6Map.Iter(); it.Next(); j++ {
		v6List[j] = Item{it.Key(), it.Val()}
	}

	return
}

func (l List) SortBy(columns ...types.ColumnIndex) List {

	// Build sorters in order
	var sorters []func(i, j Item) int
	for _, column := range columns {
		sorters = append(sorters, columnSortBy[column])
	}

	sort.Slice(l, func(i, j int) bool {

		iv, jv := l[i], l[j]

		// Run through the sorters in order
		for _, sorter := range sorters {
			if comp := sorter(iv, jv); comp != 0 {
				return comp < 0
			}
		}

		return false
	})

	return l
}

func (l AggregateList) SortBy(columns ...types.ColumnIndex) AggregateList {

	// Build sorters in order
	var sorters []func(i, j AggregateItem) int
	for _, column := range columns {
		sorters = append(sorters, columnExtSortBy[column])
	}

	sort.Slice(l, func(i, j int) bool {

		iv, jv := l[i], l[j]

		// Run through the sorters in order
		for _, sorter := range sorters {
			if comp := sorter(iv, jv); comp != 0 {
				return comp < 0
			}
		}

		return false
	})

	return l
}

// Sort orders relevant flow columns so that they become more compressible
func (l List) SortForStorage() List {
	return l.SortBy(
		types.SipColIdx,
		types.DipColIdx,
		types.DportColIdx,
		types.ProtoColIdx,
	)
}

var columnSortBy = [types.ColIdxCount]func(i, j Item) int{
	types.SipColIdx: func(i, j Item) int {
		return bytes.Compare(i.GetSip(), j.GetSip())
	},
	types.DipColIdx: func(i, j Item) int {
		return bytes.Compare(i.GetDip(), j.GetDip())
	},
	types.ProtoColIdx: func(i, j Item) int {
		if i.GetProto() < j.GetProto() {
			return -1
		} else if i.GetProto() > j.GetProto() {
			return 1
		}
		return 0
	},
	types.DportColIdx: func(i, j Item) int {
		return bytes.Compare(i.GetDport(), j.GetDport())
	},
	types.BytesRcvdColIdx: func(i, j Item) int {
		if i.BytesRcvd < j.BytesRcvd {
			return -1
		} else if i.BytesRcvd > j.BytesRcvd {
			return 1
		}
		return 0
	},
	types.BytesSentColIdx: func(i, j Item) int {
		if i.BytesSent < j.BytesSent {
			return -1
		} else if i.BytesSent > j.BytesSent {
			return 1
		}
		return 0
	},
	types.PacketsRcvdColIdx: func(i, j Item) int {
		if i.PacketsRcvd < j.PacketsRcvd {
			return -1
		} else if i.PacketsRcvd > j.PacketsRcvd {
			return 1
		}
		return 0
	},
	types.PacketsSentColIdx: func(i, j Item) int {
		if i.PacketsSent < j.PacketsSent {
			return -1
		} else if i.PacketsSent > j.PacketsSent {
			return 1
		}
		return 0
	},
}

var columnExtSortBy = [types.ExtColIdxCount]func(i, j AggregateItem) int{
	types.SipColIdx: func(i, j AggregateItem) int {
		return bytes.Compare(i.GetSip(), j.GetSip())
	},
	types.DipColIdx: func(i, j AggregateItem) int {
		return bytes.Compare(i.GetDip(), j.GetDip())
	},
	types.ProtoColIdx: func(i, j AggregateItem) int {
		if i.GetProto() < j.GetProto() {
			return -1
		} else if i.GetProto() > j.GetProto() {
			return 1
		}
		return 0
	},
	types.DportColIdx: func(i, j AggregateItem) int {
		return bytes.Compare(i.GetDport(), j.GetDport())
	},
	types.BytesRcvdColIdx: func(i, j AggregateItem) int {
		if i.BytesRcvd < j.BytesRcvd {
			return -1
		} else if i.BytesRcvd > j.BytesRcvd {
			return 1
		}
		return 0
	},
	types.BytesSentColIdx: func(i, j AggregateItem) int {
		if i.BytesSent < j.BytesSent {
			return -1
		} else if i.BytesSent > j.BytesSent {
			return 1
		}
		return 0
	},
	types.PacketsRcvdColIdx: func(i, j AggregateItem) int {
		if i.PacketsRcvd < j.PacketsRcvd {
			return -1
		} else if i.PacketsRcvd > j.PacketsRcvd {
			return 1
		}
		return 0
	},
	types.PacketsSentColIdx: func(i, j AggregateItem) int {
		if i.PacketsSent < j.PacketsSent {
			return -1
		} else if i.PacketsSent > j.PacketsSent {
			return 1
		}
		return 0
	},
	types.IfaceColIdx: func(i, j AggregateItem) int {
		return strings.Compare(i.Iface, j.Iface)
	},
	types.TimeColIdx: func(i, j AggregateItem) int {
		iTs, _ := i.AttrTime()
		jTs, _ := j.AttrTime()
		if iTs < jTs {
			return -1
		} else if iTs > jTs {
			return 1
		}
		return 0
	},
}
