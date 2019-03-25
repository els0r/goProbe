package query

import (
	"encoding/json"
	"sort"

	"github.com/els0r/goProbe/pkg/goDB"
)

// SortOrder indicates by what the entries are sorted.
type SortOrder int

const (
	SortUnknown SortOrder = iota
	SortPackets
	SortTraffic
	SortTime
)

// For the sorting we refer to closures to be able so sort by whatever value
// struct field we want
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
	return json.Marshal(s.String())
}

// UnmarshalJSON implements the Unmarshaler interface
func (s SortOrder) UnmarshalJSON(b []byte) error {
	var str string
	err := json.Unmarshal(b, &str)
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

func By(sort SortOrder, direction Direction, ascending bool) by {
	switch sort {
	case SortPackets:
		switch direction {
		case DirectionBoth, DirectionSum:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nPs+e1.nPr < e2.nPs+e2.nPr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nPs+e1.nPr > e2.nPs+e2.nPr
				}
			}
		case DirectionIn:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nPr < e2.nPr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nPr > e2.nPr
				}
			}
		case DirectionOut:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nPs < e2.nPs
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nPs > e2.nPs
				}
			}
		}
	case SortTraffic:
		switch direction {
		case DirectionBoth, DirectionSum:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nBs+e1.nBr < e2.nBs+e2.nBr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nBs+e1.nBr > e2.nBs+e2.nBr
				}
			}
		case DirectionIn:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nBr < e2.nBr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nBr > e2.nBr
				}
			}
		case DirectionOut:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nBs < e2.nBs
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nBs > e2.nBs
				}
			}
		}
	case SortTime:
		if ascending {
			return func(e1, e2 *Entry) bool {
				return e1.k.Time < e2.k.Time
			}
		} else {
			return func(e1, e2 *Entry) bool {
				return e1.k.Time > e2.k.Time
			}
		}
	}

	panic("Failed to generate Less func for sorting entries")
}
