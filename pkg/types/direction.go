package types

import jsoniter "github.com/json-iterator/go"

// Direction indicates the counters of which flow direction we should print.
type Direction int

// Enumeration of directions to be considered
const (
	DirectionUnknown Direction = iota
	DirectionSum               // sum of inbound and outbound counters
	DirectionIn                // inbound counters
	DirectionOut               // outbound counters
	DirectionBoth              // inbound and outbound counters
)

// String implement human-readable printing of the direction
func (d Direction) String() string {
	switch d {
	case DirectionSum:
		return "sum"
	case DirectionIn:
		return "in"
	case DirectionOut:
		return "out"
	case DirectionBoth:
		return "bi-directional"
	}
	return "unknown"
}

// DirectionFromString maps a string to a Direction
func DirectionFromString(s string) Direction {
	switch s {
	case "sum":
		return DirectionSum
	case "in":
		return DirectionIn
	case "out":
		return DirectionOut
	case "bi-directional":
		return DirectionOut
	}
	return DirectionUnknown
}

// MarshalJSON implements the Marshaler interface for sort order
func (d *Direction) MarshalJSON() ([]byte, error) {
	return jsoniter.Marshal(d.String())
}

// UnmarshalJSON implements the Unmarshaler interface
func (d *Direction) UnmarshalJSON(b []byte) error {
	var str string
	err := jsoniter.Unmarshal(b, &str)
	if err != nil {
		return err
	}
	*d = DirectionFromString(str)
	return nil
}
