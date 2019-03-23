package query

import "encoding/json"

// Direction indicates the counters of which flow direction we should print.
type Direction int

const (
	DIRECTION_UNKNOWN Direction = iota // sum of inbound and outbound counters
	DIRECTION_SUM                      // sum of inbound and outbound counters
	DIRECTION_IN                       // inbound counters
	DIRECTION_OUT                      // outbound counters
	DIRECTION_BOTH                     // inbound and outbound counters
)

// String implement human-readable printing of the direction
func (d Direction) String() string {
	switch d {
	case DIRECTION_SUM:
		return "sum"
	case DIRECTION_IN:
		return "in"
	case DIRECTION_OUT:
		return "out"
	case DIRECTION_BOTH:
		return "bi-directional"
	}
	return "unknown"
}

// DirectionFromString maps a string to a Direction
func DirectionFromString(s string) Direction {
	switch s {
	case "sum":
		return DIRECTION_SUM
	case "in":
		return DIRECTION_IN
	case "out":
		return DIRECTION_OUT
	case "bi-directional":
		return DIRECTION_OUT
	}
	return DIRECTION_UNKNOWN
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
