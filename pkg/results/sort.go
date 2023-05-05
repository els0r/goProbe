package results

import (
	"sort"

	"github.com/els0r/goProbe/pkg/types"
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

type by func(e1, e2 *Row) bool

type entrySorter struct {
	entries Rows
	less    func(e1, e2 *Row) bool
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
func (b by) Sort(entries []Row) {
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
func By(sort SortOrder, direction types.Direction, ascending bool) by {
	switch sort {
	case SortPackets:
		switch direction {
		case types.DirectionBoth, types.DirectionSum:
			if ascending {
				return func(e1, e2 *Row) bool {
					if e1.Counters.PacketsSent+e1.Counters.PacketsRcvd == e2.Counters.PacketsSent+e2.Counters.PacketsRcvd {
						return e1.Attributes.Less(e2.Attributes)
					}
					return e1.Counters.PacketsSent+e1.Counters.PacketsRcvd < e2.Counters.PacketsSent+e2.Counters.PacketsRcvd
				}
			}
			return func(e1, e2 *Row) bool {
				if e1.Counters.PacketsSent+e1.Counters.PacketsRcvd == e2.Counters.PacketsSent+e2.Counters.PacketsRcvd {
					return e2.Attributes.Less(e1.Attributes)
				}
				return e1.Counters.PacketsSent+e1.Counters.PacketsRcvd > e2.Counters.PacketsSent+e2.Counters.PacketsRcvd
			}
		case types.DirectionIn:
			if ascending {
				return func(e1, e2 *Row) bool {
					if e1.Counters.PacketsRcvd == e2.Counters.PacketsRcvd {
						return e1.Attributes.Less(e2.Attributes)
					}
					return e1.Counters.PacketsRcvd < e2.Counters.PacketsRcvd
				}
			}
			return func(e1, e2 *Row) bool {
				if e1.Counters.PacketsRcvd == e2.Counters.PacketsRcvd {
					return e2.Attributes.Less(e1.Attributes)
				}
				return e1.Counters.PacketsRcvd > e2.Counters.PacketsRcvd
			}
		case types.DirectionOut:
			if ascending {
				return func(e1, e2 *Row) bool {
					if e1.Counters.PacketsSent == e2.Counters.PacketsSent {
						return e1.Attributes.Less(e2.Attributes)
					}
					return e1.Counters.PacketsSent < e2.Counters.PacketsSent
				}
			}
			return func(e1, e2 *Row) bool {
				if e1.Counters.PacketsSent == e2.Counters.PacketsSent {
					return e2.Attributes.Less(e1.Attributes)
				}
				return e1.Counters.PacketsSent > e2.Counters.PacketsSent
			}
		}
	case SortTraffic:
		switch direction {
		case types.DirectionBoth, types.DirectionSum:
			if ascending {
				return func(e1, e2 *Row) bool {
					if e1.Counters.BytesSent+e1.Counters.BytesRcvd == e2.Counters.BytesSent+e2.Counters.BytesRcvd {
						return e1.Attributes.Less(e2.Attributes)
					}
					return e1.Counters.BytesSent+e1.Counters.BytesRcvd < e2.Counters.BytesSent+e2.Counters.BytesRcvd
				}
			}
			return func(e1, e2 *Row) bool {
				if e1.Counters.BytesSent+e1.Counters.BytesRcvd == e2.Counters.BytesSent+e2.Counters.BytesRcvd {
					return e2.Attributes.Less(e1.Attributes)
				}
				return e1.Counters.BytesSent+e1.Counters.BytesRcvd > e2.Counters.BytesSent+e2.Counters.BytesRcvd
			}
		case types.DirectionIn:
			if ascending {
				return func(e1, e2 *Row) bool {
					if e1.Counters.BytesRcvd == e2.Counters.BytesRcvd {
						return e1.Attributes.Less(e2.Attributes)
					}
					return e1.Counters.BytesRcvd < e2.Counters.BytesRcvd
				}
			}
			return func(e1, e2 *Row) bool {
				if e1.Counters.BytesRcvd == e2.Counters.BytesRcvd {
					return e2.Attributes.Less(e1.Attributes)
				}
				return e1.Counters.BytesRcvd > e2.Counters.BytesRcvd
			}
		case types.DirectionOut:
			if ascending {
				return func(e1, e2 *Row) bool {
					if e1.Counters.BytesSent == e2.Counters.BytesSent {
						return e1.Attributes.Less(e2.Attributes)
					}
					return e1.Counters.BytesSent < e2.Counters.BytesSent
				}
			}
			return func(e1, e2 *Row) bool {
				if e1.Counters.BytesSent == e2.Counters.BytesSent {
					return e2.Attributes.Less(e1.Attributes)
				}
				return e1.Counters.BytesSent > e2.Counters.BytesSent
			}
		}
	case SortTime:
		if ascending {
			return func(e1, e2 *Row) bool {
				if e1.Labels.Timestamp.Equal(e2.Labels.Timestamp) {
					return e1.Attributes.Less(e2.Attributes)
				}
				return e1.Labels.Timestamp.Before(e2.Labels.Timestamp)
			}
		}
		return func(e1, e2 *Row) bool {
			if e1.Labels.Timestamp.Equal(e2.Labels.Timestamp) {
				return e2.Attributes.Less(e1.Attributes)
			}
			return e1.Labels.Timestamp.After(e2.Labels.Timestamp)
		}
	}

	panic("Failed to generate Less func for sorting entries")
}
