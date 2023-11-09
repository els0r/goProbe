package types

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"log/slog"
)

// DataLimitError stores / implies that the requested data range (contained in the First / Last fields, respectively)
// exceeds the server-side limit. It provides a Next field as hint to the caller regarding the next range to be queried
// in order to achieve pagination-style partial results until all data is consumed / returned
type DataLimitError struct {
	First int64
	Last  int64
	Next  int64
}

// IsDataLimitError provides a convenience function to ascertain if a (potentially wrapped) error is of type DataLimitError
func IsDataLimitError(err error) bool {
	if err == nil {
		return false
	}

	var dataLimitErrorTarget *DataLimitError
	return errors.As(err, &dataLimitErrorTarget)
}

// NewDataLimitError instantiates a new DataLimitError according to the provided reference timestamps
func NewDataLimitError(first, last, next int64) *DataLimitError {
	return &DataLimitError{
		First: first,
		Last:  last,
		Next:  next,
	}
}

// Error implements the standard error interface
func (err *DataLimitError) Error() string {
	return fmt.Sprintf("query range (%d -> %d) exceeds internal limits, returning partial results (next chunk starts at %d)",
		err.First, err.Last, err.Next,
	)
}

// Pretty implements the Prettier interface
func (err *DataLimitError) Pretty() string {
	return fmt.Sprintf("query range (%v -> %v) exceeds internal limits, returning partial results (next chunk starts at %d)",
		time.Unix(err.First, 0).Format(DefaultTimeOutputFormat),
		time.Unix(err.Last, 0).Format(DefaultTimeOutputFormat),
		err.Next,
	)
}

// ParseError stores an error encountered during tokenized parsing
type ParseError struct {
	Tokens      []string `json:"tokens"`        // Tokens: the individual tokens parsed. Example: ["dip", "=", "1.2.3.4"]
	Pos         int      `json:"pos"`           // Pos: position at which the parser found the erorr. Example: 2
	Description string   `json:"description"`   // Description: description of the erroneous state
	Sep         string   `json:"sep,omitempty"` // Sep: separator that was used to tokenize. Example: " "
}

// RangeError stores an error where a value is not within a predefined range. It is the caller's responsibility
// to make sure that Val is actually conflicting with Min/Max. Otherwise, there's no point to instantiate the
// error in the first place
type RangeError struct {
	Val string       `json:"val,omitempty"` // Val: value that doesn't fit into the range
	Min *boundsError `json:"min"`           // Min: lower bound
	Max *boundsError `json:"max"`           // Max: upper bound
}

// MinBoundsError stores an error communicating thata a value is below a permitted value
type MinBoundsError struct {
	Val string       `json:"val,omitempty"` // Val: value that is below the lower bounds
	Min *boundsError `json:"min"`           // Min: lower bound
}

// MaxBoundsError stores an error communicating that a value is below a permitted value
type MaxBoundsError struct {
	Val string       `json:"val,omitempty"` // Val: values that is above the upper bounds
	Max *boundsError `json:"max"`           // Max: upper bound
}

// UnsupportedError stores an error communicating that a value is not included in a set of values
type UnsupportedError struct {
	Val   string   `json:"val"`   // Val: the value not part of Valid. Example: biscuits
	Valid []string `json:"valid"` // Valid: the permitted values. Example: ["csv", "json", "text"]
}

type boundsError struct {
	Includes bool   `json:"includes"` // Includes: indicates whether the value is included in the comparison or not. Example: false
	Val      string `json:"val"`      // Val: the bound. Example: 0
}

// NewParseError creates a new ParseError. The parameter "sep" will guide how tokens are re-assembled
func NewParseError(tokens []string, pos int, sep, description string, args ...any) *ParseError {
	return &ParseError{
		Tokens:      tokens,
		Pos:         pos,
		Description: fmt.Sprintf(description, args...),
		Sep:         sep,
	}
}

func (err *ParseError) parsedString() string {
	return strings.Join(err.Tokens[:err.Pos], err.Sep)
}

// Unused:
// func (err *ParseError) tokenString() string {
// 	return strings.Join(err.Tokens, err.Sep)
// }

// Error implements the standard error interface
func (err *ParseError) Error() string {

	// Reassemble the tokens.
	final := err.parsedString()
	if err.Pos > 0 {
		final += err.Sep
	}

	// Remember position of current token in reassembled string
	offset := len(final)

	// Add remaining tokens
	final += strings.Join(err.Tokens[err.Pos:], err.Sep)

	// Draw arrow
	final += "\n" + strings.Repeat(" ", offset) + "^\n"

	// Add error description
	final += err.Description
	return final
}

// Pretty implements the Prettier interface
func (err *ParseError) Pretty() string {
	return "\n" + err.Error()
}

// LogValue returns a convenience slog group that can be used for structured logging
func (err *ParseError) LogValue() slog.Value {
	attr := []slog.Attr{
		slog.Any("tokens", err.Tokens),
		slog.Int("pos", err.Pos),
		slog.String("description", err.Description),
	}
	return slog.GroupValue(attr...)
}

// NewRangeError instantiates a new RangeError
func NewRangeError(val, min string, includeMin bool, max string, includeMax bool) *RangeError {
	return &RangeError{
		Val: val,
		Min: newBoundsError(min, includeMin),
		Max: newBoundsError(max, includeMax),
	}
}

// Error implements the standard error interface
func (err *RangeError) Error() string {
	if err.Max.Val < err.Min.Val {
		return "the lower bound must not be greater than the upper bound"
	}

	var strs = []string{"("}
	if err.Min.Includes {
		strs[0] = "["
	}
	strs = append(strs, err.Min.Val, ", ", err.Max.Val)
	if err.Max.Includes {
		strs = append(strs, "]")
	} else {
		strs = append(strs, ")")
	}
	return fmt.Sprintf("range constraint not met: %v not in %s", err.Val, strings.Join(strs, ""))
}

// NewMinBoundsError instantiates a new MinBoundsError according to the provided parameters
func NewMinBoundsError(val, min string, inclusive bool) *MinBoundsError {
	return &MinBoundsError{
		Val: val,
		Min: newBoundsError(min, inclusive),
	}
}

// Error implements the standard error interface
func (err *MinBoundsError) Error() string {
	comp := ">"
	if err.Min.Includes {
		comp = "=" + comp
	}
	return fmt.Sprintf("min constraint not met: %s must be %s %s", err.Val, comp, err.Min.Val)
}

// NewMaxBoundsError instantiates a new MaxBoundsError according to the provided parameters
func NewMaxBoundsError(val, max string, inclusive bool) *MaxBoundsError {
	return &MaxBoundsError{
		Val: val,
		Max: newBoundsError(max, inclusive),
	}
}

// Error implements the standard error interface
func (err *MaxBoundsError) Error() string {
	comp := "<"
	if err.Max.Includes {
		comp += "="
	}
	return fmt.Sprintf("max constraint not met: %s must be %s %s", err.Val, comp, err.Max.Val)
}

func newBoundsError(val string, inclusive bool) *boundsError {
	return &boundsError{Val: val, Includes: inclusive}
}

// Error implements the standard error interface
func (err *UnsupportedError) Error() string {
	return fmt.Sprintf("'%s' is not in {%s}", err.Val, strings.Join(err.Valid, ", "))
}

// NewUnsupportedError instantiates a new UnsupportedError according to the provided parameters
func NewUnsupportedError(val string, valid []string) *UnsupportedError {
	return &UnsupportedError{
		Val:   val,
		Valid: valid,
	}
}
