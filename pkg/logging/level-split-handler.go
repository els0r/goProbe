package logging

import (
	"context"
	"log/slog"
)

// levelSplitHandler is able to treat error and standard output differently and route
// it to different io.Writers. sepLevel determines which level (and above) is considered
// "error". The default is LevelError
type levelSplitHandler struct {
	standard slog.Handler
	// settles above which level is the output considered an error
	sepLevel slog.Level
	errs     slog.Handler
}

func newLevelSplitHandler(std, errs slog.Handler) *levelSplitHandler {
	return &levelSplitHandler{
		standard: std,
		sepLevel: LevelError,
		errs:     errs,
	}
}

func (l *levelSplitHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if level < l.sepLevel {
		return l.standard.Enabled(ctx, level)
	}
	return l.errs.Enabled(ctx, level)
}

func (l *levelSplitHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level < l.sepLevel {
		return l.standard.Handle(ctx, r)
	}
	return l.errs.Handle(ctx, r)
}

func (l *levelSplitHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelSplitHandler{
		standard: l.standard.WithAttrs(attrs),
		sepLevel: l.sepLevel,
		errs:     l.errs.WithAttrs(attrs),
	}
}

func (l *levelSplitHandler) WithGroup(group string) slog.Handler {
	return &levelSplitHandler{
		standard: l.standard.WithGroup(group),
		sepLevel: l.sepLevel,
		errs:     l.errs.WithGroup(group),
	}
}
