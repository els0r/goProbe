package logging

import (
	"golang.org/x/exp/slog"
)

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
	LevelFatal = slog.Level(12)
	LevelPanic = slog.Level(13)
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

type L struct {
	*slog.Logger
	*formatter
}

func newL(logger *slog.Logger) *L {
	return &L{
		Logger: logger,
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
