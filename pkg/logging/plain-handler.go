package logging

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"unicode"
)

type plainHandler struct {
	mu    sync.Mutex
	w     io.Writer
	level slog.Level
}

func newPlainHandler(w io.Writer, level slog.Level) *plainHandler {
	return &plainHandler{
		mu:    sync.Mutex{},
		w:     w,
		level: level,
	}
}

func (ph *plainHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= ph.level
}

func (ph *plainHandler) Handle(ctx context.Context, r slog.Record) error {
	runes := []rune(r.Message)

	// upper case the first letter
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	runes = append(runes, '\n')

	ph.mu.Lock()
	_, err := ph.w.Write([]byte(string(runes)))
	ph.mu.Unlock()
	return err
}

func (ph *plainHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return ph
}

func (ph *plainHandler) WithGroup(_ string) slog.Handler {
	return ph
}
