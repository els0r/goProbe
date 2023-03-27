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

// Debugf allows writing of formatted debug messages to the logger
func (f *formatter) Debugf(format string, args ...interface{}) {
	if !f.l.Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	// get caller who called the function, not the function itself
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:]) // skip [Callers, Infof]

	r := slog.NewRecord(time.Now(), slog.LevelDebug, fmt.Sprintf(format, args...), pcs[0])
	_ = f.l.Handler().Handle(context.Background(), r)
}

// Infof allows writing of formatted info messages to the logger
func (f *formatter) Infof(format string, args ...interface{}) {
	if !f.l.Enabled(context.Background(), slog.LevelInfo) {
		return
	}

	// get caller who called the function, not the function itself
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:]) // skip [Callers, Infof]

	r := slog.NewRecord(time.Now(), slog.LevelInfo, fmt.Sprintf(format, args...), pcs[0])
	_ = f.l.Handler().Handle(context.Background(), r)
}

// Warnf allows writing of formatted warning messages to the logger
func (f *formatter) Warnf(format string, args ...interface{}) {
	if !f.l.Enabled(context.Background(), slog.LevelWarn) {
		return
	}

	// get caller who called the function, not the function itself
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])

	r := slog.NewRecord(time.Now(), slog.LevelWarn, fmt.Sprintf(format, args...), pcs[0])
	_ = f.l.Handler().Handle(context.Background(), r)
}

// Errorf allows writing of formatted messages to the logger. It's variadic
// arguments will _not_ add key-value pairs to the message, but be used
// as part of the msg's format string
func (f *formatter) Errorf(format string, args ...interface{}) {
	if !f.l.Enabled(context.Background(), slog.LevelError) {
		return
	}

	// get caller who called the function, not the function itself
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])

	r := slog.NewRecord(time.Now(), slog.LevelError, fmt.Sprintf(format, args...), pcs[0])
	_ = f.l.Handler().Handle(context.Background(), r)
}

// Fatal will emit a log message with level fatal and exit with a non-zero exit code. It allows
// for adding slog.Attr to it
func (f *formatter) Fatal(msg string, attr ...any) {
	if !f.l.Enabled(context.Background(), LevelFatal) {
		return
	}

	// get caller who called the function, not the function itself
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])

	r := slog.NewRecord(time.Now(), LevelFatal, msg, pcs[0])
	r.Add(attr...)
	_ = f.l.Handler().Handle(context.Background(), r)

	f.exiter.Exit(1)
}

// Fatalf will emit a formatted log message with level fatal and exit with a non-zero exit code
func (f *formatter) Fatalf(format string, args ...interface{}) {
	if !f.l.Enabled(context.Background(), LevelFatal) {
		return
	}

	// get caller who called the function, not the function itself
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])

	r := slog.NewRecord(time.Now(), LevelFatal, fmt.Sprintf(format, args...), pcs[0])
	_ = f.l.Handler().Handle(context.Background(), r)

	f.exiter.Exit(1)
}

// Panic will emit a log message with level panic and panic
func (f *formatter) Panic(msg string, attr ...any) {
	if !f.l.Enabled(context.Background(), LevelPanic) {
		return
	}

	// get caller who called the function, not the function itself
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])

	r := slog.NewRecord(time.Now(), LevelPanic, msg, pcs[0])
	r.Add(attr...)
	_ = f.l.Handler().Handle(context.Background(), r)

	f.panicker.Panic(msg)
}

// Panicf will emit a formatted log message with level panic and panic
func (f *formatter) Panicf(format string, args ...interface{}) {
	if !f.l.Enabled(context.Background(), LevelPanic) {
		return
	}

	// get caller who called the function, not the function itself
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])

	msg := fmt.Sprintf(format, args...)
	r := slog.NewRecord(time.Now(), LevelPanic, msg, pcs[0])
	_ = f.l.Handler().Handle(context.Background(), r)

	f.panicker.Panic(msg)
}
