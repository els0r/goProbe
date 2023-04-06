package logging

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"golang.org/x/exp/slog"
)

type formatter struct {
	l *slog.Logger

	exiter   exiter
	panicker panicker
}

type exiter interface {
	Exit(code int)
}

type defaultExiter struct{}

func (de defaultExiter) Exit(code int) {
	os.Exit(code)
}

type panicker interface {
	Panic(msg string)
}

type defaultPanicker struct{}

func (dp defaultPanicker) Panic(msg string) {
	panic(msg)
}

func getPc() uintptr {
	if addSource {
		// get caller who called the function, not the function itself
		var pcs [1]uintptr
		runtime.Callers(3, pcs[:]) // skip [Callers, <Log>f, getPc]

		return pcs[0]
	}
	return 0
}

var enableCtx = context.Background()

// Debug will emit a log message with level debug
func (f *formatter) Debug(args ...interface{}) {
	if !f.l.Enabled(enableCtx, slog.LevelDebug) {
		return
	}

	r := slog.NewRecord(time.Now(), slog.LevelDebug, fmt.Sprint(args...), getPc())
	_ = f.l.Handler().Handle(enableCtx, r)
}

// Debugf allows writing of formatted debug messages to the logger
func (f *formatter) Debugf(format string, args ...interface{}) {
	if !f.l.Enabled(enableCtx, slog.LevelDebug) {
		return
	}

	r := slog.NewRecord(time.Now(), slog.LevelDebug, fmt.Sprintf(format, args...), getPc())
	_ = f.l.Handler().Handle(enableCtx, r)
}

// Info will emit a log message with level warn
func (f *formatter) Info(args ...interface{}) {
	if !f.l.Enabled(enableCtx, slog.LevelInfo) {
		return
	}

	r := slog.NewRecord(time.Now(), slog.LevelInfo, fmt.Sprint(args...), getPc())
	_ = f.l.Handler().Handle(enableCtx, r)
}

// Infof allows writing of formatted info messages to the logger
func (f *formatter) Infof(format string, args ...interface{}) {
	if !f.l.Enabled(enableCtx, slog.LevelInfo) {
		return
	}

	r := slog.NewRecord(time.Now(), slog.LevelInfo, fmt.Sprintf(format, args...), getPc())
	_ = f.l.Handler().Handle(enableCtx, r)
}

// Warn will emit a log message with level warn
func (f *formatter) Warn(args ...interface{}) {
	if !f.l.Enabled(enableCtx, slog.LevelWarn) {
		return
	}

	r := slog.NewRecord(time.Now(), slog.LevelWarn, fmt.Sprint(args...), getPc())
	_ = f.l.Handler().Handle(enableCtx, r)
}

// Warnf allows writing of formatted warning messages to the logger
func (f *formatter) Warnf(format string, args ...interface{}) {
	if !f.l.Enabled(enableCtx, slog.LevelWarn) {
		return
	}

	r := slog.NewRecord(time.Now(), slog.LevelWarn, fmt.Sprintf(format, args...), getPc())
	_ = f.l.Handler().Handle(enableCtx, r)
}

// Error will emit a log message with level error
func (f *formatter) Error(args ...interface{}) {
	if !f.l.Enabled(enableCtx, slog.LevelError) {
		return
	}

	r := slog.NewRecord(time.Now(), slog.LevelError, fmt.Sprint(args...), getPc())
	_ = f.l.Handler().Handle(enableCtx, r)
}

// Errorf allows writing of formatted messages to the logger. It's variadic
// arguments will _not_ add key-value pairs to the message, but be used
// as part of the msg's format string
func (f *formatter) Errorf(format string, args ...interface{}) {
	if !f.l.Enabled(enableCtx, slog.LevelError) {
		return
	}

	r := slog.NewRecord(time.Now(), slog.LevelError, fmt.Sprintf(format, args...), getPc())
	_ = f.l.Handler().Handle(enableCtx, r)
}

// Fatal will emit a log message with level fatal and exit with a non-zero exit code
func (f *formatter) Fatal(args ...interface{}) {
	if !f.l.Enabled(enableCtx, LevelFatal) {
		return
	}

	r := slog.NewRecord(time.Now(), LevelFatal, fmt.Sprint(args...), getPc())
	_ = f.l.Handler().Handle(enableCtx, r)

	f.exiter.Exit(1)
}

// Fatalf will emit a formatted log message with level fatal and exit with a non-zero exit code
func (f *formatter) Fatalf(format string, args ...interface{}) {
	if !f.l.Enabled(enableCtx, LevelFatal) {
		return
	}

	r := slog.NewRecord(time.Now(), LevelFatal, fmt.Sprintf(format, args...), getPc())
	_ = f.l.Handler().Handle(enableCtx, r)

	f.exiter.Exit(1)
}

// Panic will emit a log message with level panic and panic
func (f *formatter) Panic(args ...interface{}) {
	if !f.l.Enabled(enableCtx, LevelPanic) {
		return
	}

	msg := fmt.Sprint(args...)

	r := slog.NewRecord(time.Now(), LevelPanic, msg, getPc())
	_ = f.l.Handler().Handle(enableCtx, r)

	f.panicker.Panic(msg)
}

// Panicf will emit a formatted log message with level panic and panic
func (f *formatter) Panicf(format string, args ...interface{}) {
	if !f.l.Enabled(enableCtx, LevelPanic) {
		return
	}

	msg := fmt.Sprintf(format, args...)

	r := slog.NewRecord(time.Now(), LevelPanic, msg, getPc())
	_ = f.l.Handler().Handle(enableCtx, r)

	f.panicker.Panic(msg)
}
