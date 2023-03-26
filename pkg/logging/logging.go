// Package logging supplies a global, structured logger
//
// It uses the [zap](https://github.com/uber-go/zap) logging library and its loggers are type
// `*zap.SugaredLogger` for ease-of-use or `*zap.Logger` for
// low-allocation logging (better performance)
package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/exp/slog"
)

type loggingConfig struct {
	enableCaller bool
	output       io.Writer
}

type Option func(*loggingConfig)

// WithOutput sets the log output
func WithOutput(w io.Writer) Option {
	return func(lc *loggingConfig) {
		lc.output = w
	}
}

// WithCaller sets whether the calling source should be logged, since the operation is
// computationally expensive
func WithCaller(b bool) Option {
	return func(lc *loggingConfig) {
		lc.enableCaller = b
	}
}

// Init initializes the global logger. The `encoding` variable sets whether content should
// be printed for console output or in JSON (for machine consumption)
func Init(version, logLevel, encoding string, opts ...Option) error {
	var level = new(slog.Level)
	err := level.UnmarshalText([]byte(logLevel))
	if err != nil {
		return fmt.Errorf("unsupported log level %q: %w", logLevel, err)
	}

	replaceFunc := func(groups []string, a slog.Attr) slog.Attr {
		// write time as ts
		switch a.Key {
		case slog.TimeKey:
			a.Key = "ts"
		case slog.LevelKey:
			// lowercase the level
			a.Value = slog.StringValue(strings.ToLower(a.Value.String()))
		case slog.SourceKey:
			a.Key = "caller"

			// only returns the pkg name, file and line number
			dir, file := filepath.Split(a.Value.String())
			a.Value = slog.StringValue(filepath.Join(filepath.Base(dir), file))
		}
		return a
	}

	cfg := &loggingConfig{
		enableCaller: true,
		output:       os.Stdout,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	hopts := slog.HandlerOptions{
		Level:       level,
		AddSource:   cfg.enableCaller,
		ReplaceAttr: replaceFunc,
	}
	var th slog.Handler
	switch strings.ToLower(encoding) {
	case "json":
		th = hopts.NewJSONHandler(cfg.output)
	case "logfmt":
		th = hopts.NewTextHandler(cfg.output)
	default:
		return fmt.Errorf("unknown encoding %q", encoding)
	}

	logger := slog.New(th.WithAttrs([]slog.Attr{slog.String("version", version)}))
	slog.SetDefault(logger)

	return nil
}

// Logger returns a low allocation logger for performance critical sections
func Logger() *slog.Logger {
	return slog.Default()
}

type loggerKeyType int

const (
	fieldsKey loggerKeyType = iota
)

type loggerFields struct {
	mu     *sync.RWMutex
	fields map[string]interface{}
}

func newLoggerFields() loggerFields {
	return loggerFields{
		mu:     &sync.RWMutex{},
		fields: make(map[string]interface{}),
	}
}

func getFields(ctx context.Context) (loggerFields, bool) {
	lf, ok := ctx.Value(fieldsKey).(loggerFields)
	return lf, ok
}

// NewContext returns a context that has extra fields added.
//
// The method is meant to be used in conjunction with WithContext that selects
// the context-enriched logger.
//
// The strength of this approach is that labels set in parent context are accessible
func NewContext(ctx context.Context, fields ...interface{}) context.Context {
	var (
		newFields loggerFields    = newLoggerFields()
		logCtx    context.Context = ctx
	)

	if ctx == nil {
		logCtx = context.Background()
	}

	// ignore malformed fields as the logging implementation wouldn't accept them anyhow
	if !(len(fields) >= 2 && (len(fields)%2 == 0)) {
		return logCtx
	}

	lf, ok := getFields(ctx)
	if ok {
		lf.mu.RLock()
		copyMap(lf.fields, newFields.fields)
		lf.mu.RUnlock()
	}

	// de-duplicate fields and add any that aren't present in the fields map yet
	for i := 1; i < len(fields); i = i + 2 {
		keyStr, ok := fields[i-1].(string)

		// skip fields that aren't a string key
		if !ok {
			continue
		}

		// either the key doesn't exist yet or it is overwritten
		newFields.fields[keyStr] = fields[i]
	}
	return context.WithValue(logCtx, fieldsKey, newFields)
}

// WithContext returns a sugared zap logger which has as much context set as possible
func WithContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return Logger()
	}
	ctxLoggerFields, ok := getFields(ctx)
	if ok {
		var fields []interface{}

		ctxLoggerFields.mu.RLock()

		// construct the fields for the logger
		keys := make([]string, len(ctxLoggerFields.fields))
		i := 0
		for k := range ctxLoggerFields.fields {
			keys[i] = k
			i++
		}
		sort.Strings(keys)
		for _, k := range keys {
			fields = append(fields, k, ctxLoggerFields.fields[k])
		}
		ctxLoggerFields.mu.RUnlock()

		return Logger().With(fields...)
	}
	return Logger()
}

func copyMap(in, out map[string]interface{}) {
	for k, v := range in {
		out[k] = v
	}
}
