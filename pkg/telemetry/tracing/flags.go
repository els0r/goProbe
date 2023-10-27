package tracing

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/els0r/goProbe/pkg/telemetry/tracing/internal/flagutil"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	otelGRPC = "otel-grpc"
	stdout   = "stdout"
)

var supportedCollectors = []string{
	otelGRPC,
	stdout,
}

// Tracing flag names and their help text. The '.' is the hierarchy delimiter
const (
	TracingKey = "tracing"

	TracingEnabledArg     string = TracingKey + ".enabled"
	TracingEnabledDefault bool   = true
	TracingEnabledHelp    string = "enable tracing"

	TracingCollectorKey string = TracingKey + ".collector"

	TracingCollectorTypeArg     string = TracingCollectorKey + ".type"
	TracingCollectorTypeDefault string = ""
	TracingCollectorTypeHelp    string = "tracing collector type"

	TracingCollectorEndpointArg     string = TracingCollectorKey + ".endpoint"
	TracingCollectorEndpointDefault string = ""
	TracingCollectorEndpointHelp    string = "endpoint collecting traces"
)

type traceFlagsConfig struct {
	enabled           bool
	collectorType     string
	collectorEndpoint string
}

func (t traceFlagsConfig) options(ctx context.Context) (opts []Option) {
	if !(t.enabled) {
		return opts
	}

	switch t.collectorType {
	case stdout:
		opts = append(opts, WithStdoutTraceExporter(true))
	case otelGRPC:
		opts = append(opts, WithGRPCExporter(ctx, traceFlags.collectorEndpoint))
	default:
		return opts
	}
	return opts
}

var traceFlags traceFlagsConfig

var registered bool
var registration = sync.Once{}

// RegisterFlags registers CLI arguments into flags. It accepts the Flagger interface, allowing for the use of pflag
// or other flag libraries such as cobra. Registration of the flags is done exactly once
func RegisterFlags(flags *flag.FlagSet) {
	registration.Do(func() {
		registered = true

		flags.Bool(TracingEnabledArg, TracingEnabledDefault, TracingEnabledHelp)
		flags.String(TracingCollectorTypeArg, TracingCollectorTypeDefault, flagutil.WithSupported(TracingCollectorTypeHelp, supportedCollectors))
		flags.String(TracingCollectorEndpointArg, TracingCollectorEndpointDefault, TracingCollectorEndpointHelp)

		// bind the flags to make sure they can be read from other sources (such as env and config files)
		_ = viper.BindPFlags(flags)
	})
}

// ShutdownFunc is a function that can be used to shutdown the tracing collector
type ShutdownFunc func(context.Context) error

var noShutdown ShutdownFunc = func(context.Context) error { return nil }

// InitFromFlags initializes tracing from the set of registered flags. Replaces the Init method
func InitFromFlags(ctx context.Context) (ShutdownFunc, error) {
	if !registered {
		fmt.Fprintf(os.Stderr, "CLI flags have not been registered. Use RegisterFlags in your command definition. Defaulting to no tracing\n") //revive:disable-line
		return noShutdown, nil
	}

	// access them through viper once to make sure their value is loaded
	traceFlags = traceFlagsConfig{
		enabled:           viper.GetBool(TracingEnabledArg),
		collectorType:     viper.GetString(TracingCollectorTypeArg),
		collectorEndpoint: viper.GetString(TracingCollectorEndpointArg),
	}

	opts := traceFlags.options(ctx)
	if len(opts) == 0 {
		return noShutdown, nil
	}

	return Init(opts...)
}
