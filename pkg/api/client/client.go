package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/fako1024/httpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
)

type DefaultClient struct {
	client  *http.Client
	timeout time.Duration

	scheme   string
	hostAddr string
	key      string

	name string

	requestLogging bool
}

type Option func(*DefaultClient)

func WithRequestLogging(b bool) Option {
	return func(c *DefaultClient) {
		c.requestLogging = true
	}
}

func WithRequestTimeout(timeout time.Duration) Option {
	return func(c *DefaultClient) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

func WithAPIKey(key string) Option {
	return func(c *DefaultClient) {
		if key != "" {
			c.key = key
		}
	}
}

func WithScheme(scheme string) Option {
	return func(c *DefaultClient) {
		if scheme != "" {
			c.scheme = scheme
		}
	}
}

func WithName(name string) Option {
	return func(c *DefaultClient) {
		if name != "" {
			c.name = fmt.Sprintf("%s/%s", name, version.Short())
		}
	}
}

const (
	defaultRequestTimeout = 30 * time.Second
	defaultClientName     = "default-client"
)

// NewDefault creates a new default client that can be used for all calls to goProbe APIs
func NewDefault(addr string, opts ...Option) *DefaultClient {
	c := &DefaultClient{
		client:   http.DefaultClient,
		scheme:   "http",
		hostAddr: addr,
		timeout:  defaultRequestTimeout,
		name:     defaultClientName,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.client.Transport = &transport{
		rt: otelhttp.NewTransport(
			http.DefaultTransport,
		),
		requestLogging: c.requestLogging,
		clientName:     c.name,
	}
	return c
}

func (c *DefaultClient) Client() *http.Client {
	return c.client
}

type transport struct {
	rt             http.RoundTripper
	requestLogging bool
	clientName     string
}

func (t *transport) setTraceID(r *http.Request) *http.Request {
	// extract the trace ID from the context if it is present
	sc := trace.SpanContextFromContext(r.Context())
	if !sc.HasTraceID() {
		// set tracing info if it isn't available in the request
		command := r.Method + " " + r.URL.String()

		var (
			err        error
			traceState trace.TraceState = trace.TraceState{}
		)

		traceState, err = traceState.Insert(t.clientName, command)
		if err == nil {
			r = r.WithContext(trace.ContextWithSpanContext(r.Context(), trace.NewSpanContext(
				trace.SpanContextConfig{TraceState: traceState},
			)))
		}
	}
	return r
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()

	r = t.setTraceID(r)
	r.Header.Set("User-Agent", t.clientName)

	// make request
	resp, err := t.rt.RoundTrip(r)
	duration := time.Since(start)

	if t.requestLogging {
		logger := logging.WithContext(r.Context()).With("req", slog.GroupValue(
			slog.String("method", r.Method),
			slog.String("url", r.URL.String()),
			slog.String("user-agent", r.UserAgent()),
			slog.Duration("duration", duration),
		))
		if resp != nil {
			logger = logger.With("resp", slog.GroupValue(
				slog.Int("status_code", resp.StatusCode),
			))
		}
		sc := trace.SpanContextFromContext(r.Context())
		if !sc.HasTraceID() {
			logger = logger.With(slog.String("traceID", sc.TraceID().String()))
		}

		if err == nil && 200 <= resp.StatusCode && resp.StatusCode < 300 {
			logger.Info("sent request")
		} else {
			if err != nil {
				logger.Errorf("failed to send request: %v", err)
			} else {
				logger.Errorf("failed to send request")
			}
		}
	}
	return resp, err
}

func (c *DefaultClient) Modify(ctx context.Context, req *httpc.Request) *httpc.Request {
	// retry any request that isn't 2xx
	req = req.RetryBackOffErrFn(func(resp *http.Response, _ error) bool {
		// if the response is nil, we should try again definitely
		if resp == nil {
			return true
		}
		return resp.StatusCode != http.StatusBadRequest
	})
	if c.timeout > 0 {
		req = req.Timeout(c.timeout)
	}
	if c.key != "" {
		req = req.Headers(map[string]string{
			"Authorization": fmt.Sprintf("digest %s", c.key),
		})
	}
	return req
}

func (c *DefaultClient) NewURL(path string) string {
	return fmt.Sprintf("%s://%s%s", c.scheme, c.hostAddr, path)
}
