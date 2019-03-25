package query

import "encoding/json"

// Direction indicates the counters of which flow direction we should print.
type Direction int

const (
	DirectionUnknown Direction = iota // sum of inbound and outbound counters
	DirectionSum                      // sum of inbound and outbound counters
	DirectionIn                       // inbound counters
	DirectionOut                      // outbound counters
	DirectionBoth                     // inbound and outbound counters
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
func (d Direction) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON implements the Unmarshaler interface
func (d Direction) UnmarshalJSON(b []byte) error {
	var str string
	err := json.Unmarshal(b, &str)
	if err != nil {
		return err
	}
	d = DirectionFromString(str)
	return nil
}
