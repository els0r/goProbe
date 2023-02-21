// Package logging supplies a global, structured logger
//
// It uses the [zap](https://github.com/uber-go/zap) logging library and its loggers are type
// `*zap.SugaredLogger` for ease-of-use or `*zap.Logger` for
// low-allocation logging (better performance)
package logging

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
		if !b {
			lc.Config.DisableStacktrace = true
		}
	}
}

// Init initializes the global logger. The `encoding` variable sets whether content should
// be printed for console output or in JSON (for machine consumption)
func Init(appName, appVersion, logLevel, encoding string, opts ...Option) error {
	switch strings.ToLower(encoding) {
	case "json", "console":
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
	loggerFieldsKey loggerKeyType = iota
)

type loggerFields map[string]interface{}

// NewContext returns a context that has extra fields added.
//
// The method is meant to be used in conjunction with WithContext that selects
// the context-enriched logger.
//
// The strength of this approach is that labels set in parent context are accessible
func NewContext(ctx context.Context, fields ...interface{}) context.Context {
	var (
		newFields loggerFields
		logCtx    context.Context = ctx
	)

	if ctx == nil {
		logCtx = context.Background()
	}

	// ignore malformed fields as the logging implementation wouldn't accept them anyhow
	if !(len(fields) >= 2 && (len(fields)%2 == 0)) {
		return logCtx
	}

	lf, ok := ctx.Value(loggerFieldsKey).(loggerFields)
	if ok {
		newFields = lf
	} else {
		newFields = make(loggerFields)
	}

	// de-duplicate fields and add any that aren't present in the fields map yet
	for i := 1; i < len(fields); i = i + 2 {
		keyStr, ok := fields[i-1].(string)

		// skip fields that aren't a string key
		if !ok {
			continue
		}

		// either the key doesn't exist yet or it is overwritten
		newFields[keyStr] = fields[i]
	}
	return context.WithValue(logCtx, loggerFieldsKey, newFields)
}

// WithContext returns a sugared zap logger which has as much context set as possible
func WithContext(ctx context.Context) *zap.SugaredLogger {
	if ctx == nil {
		return Logger()
	}
	ctxLoggerFields, ok := ctx.Value(loggerFieldsKey).(loggerFields)
	if ok {
		// construct the logger
		var fields []interface{}
		for k, v := range ctxLoggerFields {
			fields = append(fields, k, v)
		}
		return Logger().With(fields...)
	}
	return Logger()
}
