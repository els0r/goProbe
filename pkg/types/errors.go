package types

import (
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

const sep = " "

func (err *ParseError) parsedString() string {
	return strings.Join(err.Tokens[:err.Pos], sep)
}

func (err *ParseError) tokenString() string {
	return strings.Join(err.Tokens, sep)
}

func (err *ParseError) Error() string {
	// Reassemble the tokens.
	final := err.parsedString() + sep

	// Remember position of current token in reassembled string
	offset := len(final)

	// Add remaining tokens
	final += strings.Join(err.Tokens[err.Pos:], sep)

	// Draw arrow
	final += "\n" + strings.Repeat(sep, offset) + "^\n"

	// Add error description
	final += err.Description
	return final
}

func (err *ParseError) LogValue() slog.Value {
	attr := []slog.Attr{
		slog.Int("pos", err.Pos),
		slog.String("description", err.Description),
	}
	return slog.GroupValue(attr...)
}
