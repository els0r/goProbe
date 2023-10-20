package types

import (
	"fmt"
	"strings"

	"log/slog"
)

// ParseError stores an error encountered during condition parsing
type ParseError struct {
	Tokens      []string `json:"tokens"`
	Pos         int      `json:"pos"`
	Description string   `json:"description"`
	sep         string
}

// NewParseError creates a new ParseError. The parameter "sep" will guide how tokens are re-assembled
func NewParseError(tokens []string, pos int, sep, description string, args ...any) *ParseError {
	return &ParseError{
		Tokens:      tokens,
		Pos:         pos,
		Description: fmt.Sprintf(description, args...),
		sep:         sep,
	}
}

func (err *ParseError) parsedString() string {
	return strings.Join(err.Tokens[:err.Pos], err.sep)
}

func (err *ParseError) tokenString() string {
	return strings.Join(err.Tokens, err.sep)
}

func (err *ParseError) Error() string {
	// Reassemble the tokens.
	final := err.parsedString()
	if err.Pos > 0 {
		final += err.sep
	}

	// Remember position of current token in reassembled string
	offset := len(final)

	// Add remaining tokens
	final += strings.Join(err.Tokens[err.Pos:], err.sep)

	// Draw arrow
	final += "\n" + strings.Repeat(" ", offset) + "^\n"

	// Add error description
	final += err.Description
	return final
}

func (err *ParseError) LogValue() slog.Value {
	attr := []slog.Attr{
		slog.Any("tokens", err.Tokens),
		slog.Int("pos", err.Pos),
		slog.String("description", err.Description),
	}
	return slog.GroupValue(attr...)
}

type RangeError struct {
	*MinBoundsError
	*MaxBoundsError
}

func NewRangeError(min string, includeMin bool, max string, includeMax bool) *RangeError {
	return &RangeError{
		MinBoundsError: NewMinBoundsError(min, includeMin),
		MaxBoundsError: NewMaxBoundsError(max, includeMax),
	}
}

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
	return "range constraint not met: " + strings.Join(strs, "")
}

type MinBoundsError struct {
	Min *boundsError `json:"min"`
}

func NewMinBoundsError(min string, inclusive bool) *MinBoundsError {
	return &MinBoundsError{
		Min: newBoundsError(min, inclusive),
	}
}

func (err *MinBoundsError) Error() string {
	comp := "<"
	if err.Min.Includes {
		comp = "=" + comp
	}
	return fmt.Sprintf("min constraint not met: %s %s", err.Min.Val, comp)
}

type MaxBoundsError struct {
	Max *boundsError `json:"max"`
}

func NewMaxBoundsError(max string, inclusive bool) *MaxBoundsError {
	return &MaxBoundsError{
		Max: newBoundsError(max, inclusive),
	}
}

func (err *MaxBoundsError) Error() string {
	comp := "<"
	if err.Max.Includes {
		comp += "="
	}
	return fmt.Sprintf("max constraint not met: %s %s", comp, err.Max.Val)
}

type boundsError struct {
	Includes bool   `json:"includes"`
	Val      string `json:"val"`
}

func newBoundsError(val string, inclusive bool) *boundsError {
	return &boundsError{Val: val, Includes: inclusive}
}

type UnsupportedError struct {
	Val   string   `json:"val"`
	Valid []string `json:"valid"`
}

func (err *UnsupportedError) Error() string {
	return fmt.Sprintf("'%s' is not in {%s}", err.Val, strings.Join(err.Valid, ", "))
}

func NewUnsupportedError(val string, valid []string) *UnsupportedError {
	return &UnsupportedError{
		Val:   val,
		Valid: valid,
	}
}
