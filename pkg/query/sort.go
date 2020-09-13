package query

import (
	"sort"

	"github.com/els0r/goProbe/pkg/goDB"
	jsoniter "github.com/json-iterator/go"
)

// SortOrder indicates by what the entries are sorted.
type SortOrder int

// Enumeration of different sort orders
const (
	SortUnknown SortOrder = iota
	SortPackets
	SortTraffic
	SortTime
)

// Entry stores the fields after which we sort (bytes or packets)
type Entry struct {
	k        goDB.ExtraKey
	nBr, nBs uint64
	nPr, nPs uint64
}

type by func(e1, e2 *Entry) bool

type entrySorter struct {
	entries []Entry
	less    func(e1, e2 *Entry) bool
}

// String implement human-readable printing of the sort order
func (s SortOrder) String() string {
	switch s {
	case SortPackets:
		return "packets"
	case SortTraffic:
		return "bytes"
	case SortTime:
		return "time"
	}
	return "unknown"
}

// SortOrderFromString is the inverse operation to SortOrder.String()
func SortOrderFromString(s string) SortOrder {
	switch s {
	case "packets":
		return SortPackets
	case "bytes":
		return SortTraffic
	case "time":
		return SortTime
	}
	return SortUnknown
}

// MarshalJSON implements the Marshaler interface for sort order
func (s SortOrder) MarshalJSON() ([]byte, error) {
	return jsoniter.Marshal(s.String())
}

// UnmarshalJSON implements the Unmarshaler interface
func (s SortOrder) UnmarshalJSON(b []byte) error {
	var str string
	err := jsoniter.Unmarshal(b, &str)
	if err != nil {
		return err
	}
	s = SortOrderFromString(str)
	return nil
}

// Sort is a method on the function type, By, that sorts the argument slice according to the function
func (b by) Sort(entries []Entry) {
	es := &entrySorter{
		entries: entries,
		less:    b, // closure for sort order defintion
	}
	sort.Sort(es)
}

// Len is part of sort.Interface.
func (s *entrySorter) Len() int {
	return len(s.entries)
}

// Swap is part of sort.Interface.
func (s *entrySorter) Swap(i, j int) {
	s.entries[i], s.entries[j] = s.entries[j], s.entries[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *entrySorter) Less(i, j int) bool {
	return s.less(&s.entries[i], &s.entries[j])
}

// By is part of the sort.Interface
func By(sort SortOrder, direction Direction, ascending bool) by {
	switch sort {
	case SortPackets:
		switch direction {
		case DirectionBoth, DirectionSum:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nPs+e1.nPr < e2.nPs+e2.nPr
				}
			}
			return func(e1, e2 *Entry) bool {
				return e1.nPs+e1.nPr > e2.nPs+e2.nPr
			}
		case DirectionIn:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nPr < e2.nPr
				}
			}
			return func(e1, e2 *Entry) bool {
				return e1.nPr > e2.nPr
			}
		case DirectionOut:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nPs < e2.nPs
				}
			}
			return func(e1, e2 *Entry) bool {
				return e1.nPs > e2.nPs
			}
		}
	case SortTraffic:
		switch direction {
		case DirectionBoth, DirectionSum:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nBs+e1.nBr < e2.nBs+e2.nBr
				}
			}
			return func(e1, e2 *Entry) bool {
				return e1.nBs+e1.nBr > e2.nBs+e2.nBr
			}
		case DirectionIn:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nBr < e2.nBr
				}
			}
			return func(e1, e2 *Entry) bool {
				return e1.nBr > e2.nBr
			}
		case DirectionOut:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nBs < e2.nBs
				}
			}
			return func(e1, e2 *Entry) bool {
				return e1.nBs > e2.nBs
			}
		}
	case SortTime:
		if ascending {
			return func(e1, e2 *Entry) bool {
				return e1.k.Time < e2.k.Time
			}
		}
		return func(e1, e2 *Entry) bool {
			return e1.k.Time > e2.k.Time
		}
	}

	panic("Failed to generate Less func for sorting entries")
}
