package query

import (
	"sort"

	"github.com/els0r/goProbe/pkg/goDB"
)

// SortOrder indicates by what the entries are sorted.
type SortOrder int

const (
	SORT_PACKETS SortOrder = iota
	SORT_TRAFFIC
	SORT_TIME
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
	case SORT_PACKETS:
		switch direction {
		case DIRECTION_BOTH, DIRECTION_SUM:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nPs+e1.nPr < e2.nPs+e2.nPr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nPs+e1.nPr > e2.nPs+e2.nPr
				}
			}
		case DIRECTION_IN:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nPr < e2.nPr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nPr > e2.nPr
				}
			}
		case DIRECTION_OUT:
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
	case SORT_TRAFFIC:
		switch direction {
		case DIRECTION_BOTH, DIRECTION_SUM:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nBs+e1.nBr < e2.nBs+e2.nBr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nBs+e1.nBr > e2.nBs+e2.nBr
				}
			}
		case DIRECTION_IN:
			if ascending {
				return func(e1, e2 *Entry) bool {
					return e1.nBr < e2.nBr
				}
			} else {
				return func(e1, e2 *Entry) bool {
					return e1.nBr > e2.nBr
				}
			}
		case DIRECTION_OUT:
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
	case SORT_TIME:
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
