// Package logging supplies a global, structured logger
package logging

import (
	"context"
	"errors"
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
	stdOutput    io.Writer
	errsOutput   io.Writer
	initialAttr  map[string]slog.Attr
}

const (
	initKeyName    = "name"
	initKeyVersion = "version"
)

type Option func(*loggingConfig) error

// WithOutput sets the log output
func WithOutput(w io.Writer) Option {
	return func(lc *loggingConfig) error {
		lc.stdOutput = w
		return nil
	}
}

// WithErrorOutput sets the log output for level Error, Fatal and Panic. For the rest,
// the default output os.Stdout or the output set by `WithOutput` is chosen
func WithErrorOutput(w io.Writer) Option {
	return func(lc *loggingConfig) error {
		lc.errsOutput = w
		return nil
	}
}

var (
	emptyFilePathError = errors.New("empty filepath provided")
)

const (
	devnullOutput = "devnull"
	stderrOutput  = "stderr"
	stdoutOutput  = "stdout"
)

// WithFileOutput sets the log output to a file. The filepath can be one of the following:
//
// - stdout: logs will be written to os.Stdout
// - stderr: logs will be written to os.Stderr
// - devnull: logs will be discarded
// - any other filepath: logs will be written to the file
//
// The special filepaths are case insensitive, e.g. DEVNULL works just as well
func WithFileOutput(filepath string) Option {
	return func(lc *loggingConfig) error {
		var output io.Writer
		switch strings.ToLower(filepath) { // ToLower will allow users to pass STDERR for example
		case stdoutOutput:
			output = os.Stdout
		case stderrOutput:
			output = os.Stderr
		case devnullOutput:
			output = io.Discard
		case "":
			return emptyFilePathError
		default:
			f, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			output = f
		}
		return WithOutput(output)(lc)
	}
}

// WithCaller sets whether the calling source should be logged, since the operation is
// computationally expensive
func WithCaller(b bool) Option {
	return func(lc *loggingConfig) error {
		lc.enableCaller = b
		return nil
	}
}

// WithName sets the application name as initial field present in all log messages
func WithName(name string) Option {
	return func(lc *loggingConfig) error {
		lc.initialAttr[initKeyName] = slog.String(initKeyName, name)
		return nil
	}
}

// WithVersion sets the application version as initial field present in all log messages
func WithVersion(version string) Option {
	return func(lc *loggingConfig) error {
		lc.initialAttr[initKeyVersion] = slog.String(initKeyVersion, version)
		return nil
	}
}

// Init initializes the global logger. The `encoding` variable sets whether content should
// be printed for console output or in JSON (for machine consumption)
func Init(level slog.Level, encoding Encoding, opts ...Option) error {
	// assign configured logger to slog's default logger
	logger, err := New(level, encoding, opts...)
	if err != nil {
		return err
	}
	slog.SetDefault(logger.l)
	return nil
}

// New returns a new logger
func New(level slog.Level, encoding Encoding, opts ...Option) (*L, error) {
	if level == LevelUnknown {
		return nil, fmt.Errorf("unknown log level provided: %s", level)
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

			source := a.Value.Any().(*slog.Source)

			// only returns the pkg name, file and line number
			dir, file := filepath.Split(source.File)
			source.File = filepath.Join(filepath.Base(dir), file)
		}
		return a
	}

	cfg := &loggingConfig{
		stdOutput:   os.Stdout,
		initialAttr: make(map[string]slog.Attr),
	}
	for _, opt := range opts {
		err := opt(cfg)
		if err != nil {
			return nil, err
		}
	}

	hopts := slog.HandlerOptions{
		Level:       level,
		AddSource:   cfg.enableCaller,
		ReplaceAttr: replaceFunc,
	}

	th, err := getHandler(cfg.stdOutput, encoding, hopts)
	if err != nil {
		return nil, err
	}

	// inject a split level handler in case the error output is defined
	if cfg.errsOutput != nil {
		h, _ := getHandler(cfg.errsOutput, encoding, hopts)
		th = newLevelSplitHandler(th, h)
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

	// return a new L logger
	return newL(slog.New(th)), nil
}

func getHandler(w io.Writer, encoding Encoding, hopts slog.HandlerOptions) (th slog.Handler, err error) {
	switch encoding {
	case EncodingJSON:
		th = slog.NewJSONHandler(w, &hopts)
	case EncodingLogfmt:
		th = slog.NewTextHandler(w, &hopts)
	case EncodingPlain:
		th = newPlainHandler(w, hopts.Level.Level())
	default:
		return nil, fmt.Errorf("unknown encoding %q", encoding)
	}
	return th, nil
}

// NewFromContext creates a new logger, deriving structured fields from the supplied context
func NewFromContext(ctx context.Context, level slog.Level, encoding Encoding, opts ...Option) (*L, error) {
	logger, err := New(level, encoding, opts...)
	if err != nil {
		return nil, err
	}
	return fromContext(ctx, logger), nil
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

// WithFields returns a context that has extra fields added.
//
// The method is meant to be used in conjunction with WithContext that selects
// the context-enriched logger.
//
// The strength of this approach is that labels set in parent context are accessible
func WithFields(ctx context.Context, fields ...slog.Attr) context.Context {
	var (
		newFields loggerFields = newLoggerFields()
	)

	if ctx == nil {
		ctx = context.Background()
	}

	lf, ok := getFields(ctx)
	if ok {
		lf.mu.RLock()
		copyMap(lf.fields, newFields.fields)
		lf.mu.RUnlock()
	}

	// de-duplicate fields and add any that aren't present in the fields map yet
	for _, field := range fields {
		// either the key doesn't exist yet or it is overwritten
		newFields.fields[field.Key] = field
	}
	return context.WithValue(ctx, fieldsKey, newFields)
}

func fromContext(ctx context.Context, logger *L) *L {
	if ctx == nil {
		return logger
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
			fields = append(fields, ctxLoggerFields.fields[k])
		}
		ctxLoggerFields.mu.RUnlock()

		return logger.With(fields...)
	}
	return logger
}

// FromContext returns a global logger which has as much context set as possible
func FromContext(ctx context.Context) *L {
	return fromContext(ctx, Logger())
}

func copyMap(in, out map[string]interface{}) {
	for k, v := range in {
		out[k] = v
	}
}
