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
	loggerKey loggerKeyType = iota
	loggerFieldsKey
)

// NewContext returns a context that has a zap logger with extra fields added.
// The method is meant to be used in conjunction with WithContext that selects
// the context-enriched logger.
// The strength of this approach is that labels set in parent context are accessible
//
// NewContext _does not work_ in conjuction with WithFastContext. Use WithContext instead.
func NewContext(ctx context.Context, fields ...interface{}) context.Context {
	// TODO: this idiot will just append identical keys ....
	// FIXME
	return context.WithValue(ctx, loggerKey, WithContext(ctx).With(fields...))
}

// WithContext returns a sugared zap logger which has as much context set as possible
func WithContext(ctx context.Context) *zap.SugaredLogger {
	if ctx == nil {
		return Logger()
	}
	ctxLogger, ok := ctx.Value(loggerKey).(*zap.SugaredLogger)
	if ok {
		return ctxLogger
	}
	return Logger()
}
