package logging

import (
	"strings"

	"golang.org/x/exp/slog"
)

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
	LevelFatal = slog.Level(12)
	LevelPanic = slog.Level(13)

	// LevelUnknown specifies a level that the logger won't handle
	LevelUnknown = slog.Level(-255)
)

// Encoding defines the type of log message encoding
type Encoding string

const (
	// EncodingJSON encodes structured log messages as JSON
	// It uses the slog.JSONHandler under the hood
	EncodingJSON Encoding = "json"

	// EncodingLogfmt will output the messages in structured key=value pairs according
	// to the logfmt "standard"
	// It uses the slog.TextHandler under the hood
	EncodingLogfmt Encoding = "logfmt"

	// EncodingPlain causes only the message part of the log line to be printed
	// with the first letter of the message capitalized. It will not print any
	// other structured fields.
	//
	// The only thing setting it aside from fmt.Fprintln is that it respects the log level
	// it was initialized with
	EncodingPlain Encoding = "plain"
)

// enumeration of level keys (for performance. See Init's replaceFunc)
const (
	debugLevel = "debug"
	infoLevel  = "info"
	warnLevel  = "warn"
	errorLevel = "error"
	fatalLevel = "fatal"
	panicLevel = "panic"
)

// LevelFromString returns an slog.Level if the string matches one
// of the level's string descriptions. Otherwise the level LevelUnknown
// is returned (which won't be processed by the logger as a valid level)
func LevelFromString(lvl string) slog.Level {
	switch strings.ToLower(lvl) {
	case debugLevel:
		return LevelDebug
	case infoLevel:
		return LevelInfo
	case warnLevel:
		return LevelWarn
	case errorLevel:
		return LevelError
	case fatalLevel:
		return LevelFatal
	case panicLevel:
		return LevelPanic
	}
	return LevelUnknown
}

type L struct {
	*formatter
}

// With runs With(args...) on the slog.Logger and attaches it
func (l *L) With(args ...interface{}) *L {
	return newL(l.formatter.l.With(args...)).
		exiter(l.formatter.exiter).
		panicker(l.formatter.panicker)
}

// WithGroup runs WithGroup(group) on the slog.Logger and attaches it
func (l *L) WithGroup(group string) *L {
	return newL(l.formatter.l.WithGroup(group)).
		exiter(l.formatter.exiter).
		panicker(l.formatter.panicker)
}

func newL(logger *slog.Logger) *L {
	return &L{
		formatter: &formatter{
			l:        logger,
			exiter:   defaultExiter{},
			panicker: defaultPanicker{},
		}}
}

func (l *L) exiter(e exiter) *L {
	l.formatter.exiter = e
	return l
}

func (l *L) panicker(p panicker) *L {
	l.formatter.panicker = p
	return l
}
