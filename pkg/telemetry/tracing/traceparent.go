package tracing

import (
	"context"
	"strings"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	b3SingleHeaderKey      = "b3"
	w3traceparentHeaderKey = "traceparent"
)

// GetW3CTraceparentHeader will return the [W3C Trace Context](https://www.w3.org/TR/trace-context/) compliant
// traceparent header as a string
func GetW3CTraceparentHeader(ctx context.Context) (traceparentHeader string) {
	if ctx == nil {
		return traceparentHeader
	}

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return carrier[w3traceparentHeaderKey]
}

// GetB3SingleHeader will return the [B3 single header format](https://github.com/openzipkin/b3-propagation#single-header) compliant
// b3 header as a string
func GetB3SingleHeader(ctx context.Context) (b3header string) {
	if ctx == nil {
		return b3header
	}

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return carrier[b3SingleHeaderKey]
}

// GetTraceparentHeader will return either a W3C compliant or B3 compliant header string,
// depending on which propagator was used to create the tracing context
func GetTraceparentHeader(ctx context.Context) (traceparentHeader string) {
	// check w3c first
	traceparentHeader = GetW3CTraceparentHeader(ctx)
	if traceparentHeader != "" {
		return traceparentHeader
	}
	return GetB3SingleHeader(ctx)
}

// ContextFromW3CTraceparentHeader will inject a tracing context that will use traceparent as the root span. It is
// meant to be used for cross-cutting injection (such as from pub/sub systems, CLI tools, etc.)
func ContextFromW3CTraceparentHeader(ctx context.Context, traceparentHeader string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if traceparentHeader == "" {
		return context.Background()
	}

	p := propagation.TraceContext{}
	otel.SetTextMapPropagator(p)

	return p.Extract(ctx,
		propagation.MapCarrier{w3traceparentHeaderKey: traceparentHeader},
	)
}

// ContextFromB3SingleHeader will inject a tracing context that will use traceparent as the root span. The traceparent
// will have to comply with the [B3 single header](https://github.com/openzipkin/b3-propagation#single-header) specification
func ContextFromB3SingleHeader(ctx context.Context, b3Header string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if b3Header == "" {
		return context.Background()
	}

	p := b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader | b3.B3SingleHeader))
	otel.SetTextMapPropagator(p)

	return p.Extract(ctx, propagation.MapCarrier{b3SingleHeaderKey: b3Header})
}

// ContextFromTraceparentHeader accepts both W3C compliant and B3 single header compliant
// strings to synthesize a tracing context from an existing context
//
// The function is meant to be called at program/processing start:
//
//	rootCtx := ContextFromTraceparentHeader(context.Background(), traceparent)
//
//	// do some work
//	A(rootCtx)
func ContextFromTraceparentHeader(ctx context.Context, traceparentHeader string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	// identify whether the header is b3 or w3c
	parts := strings.Split(traceparentHeader, "-")
	if len(parts) == 4 {
		switch len(parts[0]) {
		// look for version string 00 at start of traceparent
		case 2:
			return ContextFromW3CTraceparentHeader(ctx, traceparentHeader)
		default:
			return ContextFromB3SingleHeader(ctx, traceparentHeader)
		}
	}
	return ctx
}

// GetTraceID extracts the traceID from the context if it is set
func GetTraceID(ctx context.Context) (traceID string) {
	if ctx == nil {
		return traceID
	}
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return traceID
}

// GetSpanID extracts the spanID from the context if it is set
func GetSpanID(ctx context.Context) (spanID string) {
	if ctx == nil {
		return spanID
	}
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		return sc.SpanID().String()
	}
	return spanID
}

// GetIDs returns the traceID and spanID from the context if they are set
func GetIDs(ctx context.Context) (traceID, spanID string) {
	if ctx == nil {
		return traceID, spanID
	}
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		return sc.TraceID().String(), sc.SpanID().String()
	}
	return traceID, spanID
}
