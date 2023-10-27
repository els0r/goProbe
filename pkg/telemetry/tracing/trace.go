// Package tracing supplies access to tracer providers.
//
// It uses the [otel](https://opentelemetry.io/docs/instrumentation/go/getting-started/) tracing library, connecting
// to an OpenTelemetry collector.
//
// The package relies on, and is meant to be used in conjunction with the observability package
package tracing

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	instrumentationCodeProviderName = "github.com/els0r/goProbe/pkg/tracing"
)

type tracingConfig struct {
	exporter sdktrace.SpanExporter
}

// Option allows to configure the tracing setup
type Option func(*tracingConfig) error

// WithGRPCExporter sets up an exporter using a gRPC connection to the trace collector
func WithGRPCExporter(ctx context.Context, collectorEndpoint string) Option {
	return func(tc *tracingConfig) error {
		if collectorEndpoint == "" {
			return fmt.Errorf("no collector endpoint set")
		}
		// If the OpenTelemetry Collector is running on a local cluster (minikube or
		// microk8s), it should be accessible through the NodePort service at the
		// `localhost:30080` endpoint. Otherwise, replace `localhost` with the
		// endpoint of your cluster. If you run the app inside k8s, then you can
		// probably connect directly to the service through dns.
		dialCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		conn, err := grpc.DialContext(dialCtx, collectorEndpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return fmt.Errorf("failed to create gRPC connection to collector: %w", err)
		}
		// Set up a trace exporter
		traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
		if err != nil {
			return fmt.Errorf("failed to create gRPC trace exporter: %w", err)
		}
		tc.exporter = traceExporter
		return nil
	}
}

// WithStdoutTraceExporter sets the exporter to write to stdout. It's not recommended to
// use this exporter in production and only for testing and validation
//
// A writer can be optionally provided. The default case is stdout
func WithStdoutTraceExporter(prettyPrint bool, w ...io.Writer) Option { //revive:disable-line
	return func(tc *tracingConfig) error {
		var opts []stdouttrace.Option
		if len(w) > 0 {
			opts = append(opts, stdouttrace.WithWriter(w[0]))
		}
		if prettyPrint {
			opts = append(opts, stdouttrace.WithPrettyPrint())
		}
		exp, err := stdouttrace.New(opts...)

		if err != nil {
			return fmt.Errorf("failed to create stdout trace exporter: %w", err)
		}
		tc.exporter = exp
		return nil
	}
}

// WithSpanExporter sets the exporter
func WithSpanExporter(exporter sdktrace.SpanExporter) Option {
	return func(tc *tracingConfig) error {
		tc.exporter = exporter
		return nil
	}
}

// Init initializes the tracer provider. The function is meant to be called once upon
// program setup. The reason for providing the serviceName explicitly is so that
// this package does not depend on the observability package to be intitialized
//
// Best practice is to use the serviceName from the package, having previously called
// `observability.Init`.
func Init(opts ...Option) (ShutdownFunc, error) {
	tracerProvider, err := NewTracerProvider(opts...)
	if err != nil {
		return noShutdown, fmt.Errorf("failed to create tracer provider: %w", err)
	}

	// it is imperative that this is set, since Start relies on the global tracer provider to be set.
	// Otherwise, trace propagation will not work
	otel.SetTracerProvider(tracerProvider)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Shutdown will flush any remaining spans and shut down the exporter.
	return tracerProvider.Shutdown, nil
}

// NewTracerProvider creates a new tracer provider using the provided service name. The options
// can and should be used to supply a span exporter. There is no default set on purpose, since
// the choice of exporter highly depends on the environment the application is deployed in
func NewTracerProvider(opts ...Option) (tp *sdktrace.TracerProvider, err error) {
	// apply options
	tracingCfg := &tracingConfig{}
	for _, opt := range opts {
		err = opt(tracingCfg)
		if err != nil {
			return nil, err
		}
	}
	exporter := tracingCfg.exporter
	if exporter == nil {
		return nil, fmt.Errorf("no span exporter set")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		// this will run all standard detectors to add service name and version to the resource
		sdktrace.WithResource(resource.Default()),
		sdktrace.WithSpanProcessor(bsp),
	)
	return tp, nil
}

// Start gets the tracer provider and starts a span with the provided name and options. Additional observability attributes
// from the context are added to the span with trace.WithAttributes
func Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.GetTracerProvider().Tracer(instrumentationCodeProviderName)

	if ctx == nil {
		return tracer.Start(context.Background(), spanName, opts...)
	}

	return tracer.Start(ctx, spanName, opts...)
}

// Error records the error as an event and sets the span status to error
func Error(span trace.Span, err error) {
	if err == nil {
		return
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
