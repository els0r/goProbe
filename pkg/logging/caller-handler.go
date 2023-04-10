package logging

import (
	"context"
	"runtime"

	"golang.org/x/exp/slog"
)

type callerHandler struct {
	addSource bool
	next      slog.Handler
}

func (c *callerHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return c.next.Enabled(ctx, level)
}

func (c *callerHandler) Handle(ctx context.Context, r slog.Record) error {
	if c.addSource {
		// get caller who called the function, not the function itself
		var pcs [1]uintptr
		runtime.Callers(3, pcs[:]) // skip [Callers, <Log>f, callerHandler.Handle]

		// assign program counter
		r.PC = pcs[0]
	}
	return c.next.Handle(ctx, r)
}

func (c *callerHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &callerHandler{
		addSource: c.addSource,
		next:      c.next.WithAttrs(attrs),
	}
}

func (c *callerHandler) WithGroup(group string) slog.Handler {
	return &callerHandler{
		addSource: c.addSource,
		next:      c.next.WithGroup(group),
	}
}
