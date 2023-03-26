// Package logging supplies a global, structured logger
//
// It uses the [zap](https://github.com/uber-go/zap) logging library and its loggers are type
// `*zap.SugaredLogger` for ease-of-use or `*zap.Logger` for
// low-allocation logging (better performance)
package logging

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	_ "github.com/jsternberg/zap-logfmt"
)

type loggingConfig struct {
	*zap.Config
}

type Option func(*loggingConfig)

// WithDevelopmentMode enables/disables a logger suitable for DEV environments where
// stacktraces are added more liberally
func WithDevelopmentMode(b bool) Option {
	return func(lc *loggingConfig) {
		lc.Config.Development = b
	}
}

// WithStackTraces enables/disables stacktraces logged under the "stacktraces" key
func WithStackTraces(b bool) Option {
	return func(lc *loggingConfig) {
		lc.Config.DisableStacktrace = !b
	}
}

// WithOutputPaths sets to which paths level "info" and above shall be sent. Treats "stdout" and "stderr" as
// special paths
func WithOutputPaths(paths []string) Option {
	return func(lc *loggingConfig) {
		lc.Config.OutputPaths = paths
	}
}

// WithErrorPaths sets to which paths level "error" and below shall be sent. Treats "stdout" and "stderr" as
// special paths
func WithErrorPaths(paths []string) Option {
	return func(lc *loggingConfig) {
		lc.Config.ErrorOutputPaths = paths
	}
}

// Init initializes the global logger. The `encoding` variable sets whether content should
// be printed for console output or in JSON (for machine consumption)
func Init(appName, appVersion, logLevel, encoding string, opts ...Option) error {
	switch strings.ToLower(encoding) {
	case "json", "console", "logfmt":
	default:
		return fmt.Errorf("unknown encoding %q", encoding)
	}

	var level zap.AtomicLevel
	switch strings.ToLower(logLevel) {
	case "debug":
		level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	case "fatal":
		level = zap.NewAtomicLevelAt(zapcore.FatalLevel)
	case "panic":
		level = zap.NewAtomicLevelAt(zapcore.PanicLevel)
	default:
		return fmt.Errorf("unsupported log level %q", logLevel)
	}

	zapEncoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// registering the encoder is taken care of the by teh zaplogfmt library itself
	if encoding == "logfmt" {
		zapEncoderConfig.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString(ts.UTC().Format(time.RFC3339))
		}
	}

	cfg := zap.Config{
		Level: level,
		InitialFields: map[string]interface{}{
			"app_name":    appName,
			"app_version": appVersion,
		},
		Encoding:         encoding,
		EncoderConfig:    zapEncoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// apply functional options
	lcfg := &loggingConfig{Config: &cfg}
	for _, opt := range opts {
		opt(lcfg)
	}

	logger, err := lcfg.Config.Build()
	if err != nil {
		return err
	}
	_ = zap.ReplaceGlobals(logger)

	return nil
}

// Logger returns a low allocation logger for performance
// critical sections
func Logger() *zap.SugaredLogger {
	return zap.S()
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
func WithContext(ctx context.Context) *zap.SugaredLogger {
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
