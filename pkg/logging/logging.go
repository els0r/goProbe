// Package logging supplies a global, structured logger
package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"golang.org/x/exp/slog"
)

type loggingConfig struct {
	enableCaller bool
	stdOutput    io.Writer
	errsOutput   io.Writer
	initialAttr  map[string]slog.Attr
}

const (
	initKeyName    = "name"
	initKeyVersion = "version"
)

type Option func(*loggingConfig)

// WithOutput sets the log output
func WithOutput(w io.Writer) Option {
	return func(lc *loggingConfig) {
		lc.stdOutput = w
	}
}

// WithErrorOutput sets the log output for level Error, Fatal and Panic. For the rest,
// the default output os.Stdout or the output set by `WithOutput` is chosen
func WithErrorOutput(w io.Writer) Option {
	return func(lc *loggingConfig) {
		lc.errsOutput = w
	}
}

// WithCaller sets whether the calling source should be logged, since the operation is
// computationally expensive
func WithCaller(b bool) Option {
	return func(lc *loggingConfig) {
		lc.enableCaller = b
	}
}

// WithName sets the application name as initial field present in all log messages
func WithName(name string) Option {
	return func(lc *loggingConfig) {
		lc.initialAttr[initKeyName] = slog.String(initKeyName, name)
	}
}

// WithVersion sets the application version as initial field present in all log messages
func WithVersion(version string) Option {
	return func(lc *loggingConfig) {
		lc.initialAttr[initKeyVersion] = slog.String(initKeyVersion, version)
	}
}

// Init initializes the global logger. The `encoding` variable sets whether content should
// be printed for console output or in JSON (for machine consumption)
func Init(level slog.Level, encoding Encoding, opts ...Option) error {
	if level == LevelUnknown {
		return fmt.Errorf("unknown log level provided: %s", level)
	}

	replaceFunc := func(groups []string, a slog.Attr) slog.Attr {
		// write time as ts
		switch a.Key {
		case slog.TimeKey:
			a.Key = "ts"
		case slog.LevelKey:
			// Handle custom level values
			level := a.Value.Any().(slog.Level)

			switch {
			case level < LevelInfo:
				a.Value = slog.StringValue(debugLevel)
			case level < LevelWarn:
				a.Value = slog.StringValue(infoLevel)
			case level < LevelError:
				a.Value = slog.StringValue(warnLevel)
			case level < LevelFatal:
				a.Value = slog.StringValue(errorLevel)
			case level < LevelPanic:
				a.Value = slog.StringValue(fatalLevel)
			default:
				a.Value = slog.StringValue(panicLevel)
			}
		case slog.SourceKey:
			a.Key = "caller"

			// only returns the pkg name, file and line number
			dir, file := filepath.Split(a.Value.String())
			a.Value = slog.StringValue(filepath.Join(filepath.Base(dir), file))
		}
		return a
	}

	cfg := &loggingConfig{
		stdOutput:   os.Stdout,
		initialAttr: make(map[string]slog.Attr),
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
	switch encoding {
	case EncodingJSON:
		th = hopts.NewJSONHandler(cfg.stdOutput)
	case EncodingLogfmt:
		th = hopts.NewTextHandler(cfg.stdOutput)
	default:
		return fmt.Errorf("unknown encoding %q", encoding)
	}

	// inject a split level handler in case the error output is defined
	if cfg.errsOutput != nil {
		var errth slog.Handler
		switch encoding {
		case EncodingJSON:
			errth = hopts.NewJSONHandler(cfg.errsOutput)
		case EncodingLogfmt:
			errth = hopts.NewTextHandler(cfg.errsOutput)
		}
		th = newLevelSplitHandler(th, errth)
	}

	// assign initial attributes if there are any
	var attrs []slog.Attr
	for _, attr := range cfg.initialAttr {
		attrs = append(attrs, attr)
	}

	if len(attrs) > 0 {
		sort.SliceStable(attrs, func(i, j int) bool {
			return attrs[i].Key < attrs[j].Key
		})
		th = th.WithAttrs(attrs)
	}

	if cfg.enableCaller {
		// inject a caller handler in case the caller should be reported. It's important that
		// this one comes at the beginning of the chain
		th = &callerHandler{addSource: cfg.enableCaller, next: th}
	}

	// assign configured logger to slog's default logger
	slog.SetDefault(slog.New(th))
	return nil
}

// Logger returns a low allocation logger for performance critical sections
func Logger() *L {
	return newL(slog.Default())
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
		newFields loggerFields = newLoggerFields()
	)

	if ctx == nil {
		ctx = context.Background()
	}

	// ignore malformed fields as the logging implementation wouldn't accept them anyhow
	if !(len(fields) >= 2 && (len(fields)%2 == 0)) {
		return ctx
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
	return context.WithValue(ctx, fieldsKey, newFields)
}

// WithContext returns a logger which has as much context set as possible
func WithContext(ctx context.Context) *L {
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
