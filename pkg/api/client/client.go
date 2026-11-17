package client

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/api/server"
	"github.com/els0r/goProbe/v4/pkg/goDB/info"
	"github.com/els0r/goProbe/v4/pkg/version"
	"github.com/els0r/telemetry/logging"
	"github.com/fako1024/httpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

// DefaultClient denotes the default client used for all requests
type DefaultClient struct {
	client  *http.Client
	baseRT  http.RoundTripper
	timeout time.Duration

	retry          bool
	retryIntervals httpc.Intervals

	scheme   string
	hostAddr string
	key      string
	token    string

	name string

	requestLogging bool
}

// Option configures the client
type Option func(*DefaultClient)

// WithRequestLogging enables logging of client requests
func WithRequestLogging(b bool) Option {
	return func(c *DefaultClient) {
		c.requestLogging = b
	}
}

// WithRequestTimeout sets the timeout for every request
func WithRequestTimeout(timeout time.Duration) Option {
	return func(c *DefaultClient) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

// WithAPIKey sets the API key to be presented to the API server
func WithAPIKey(key string) Option {
	return func(c *DefaultClient) {
		if key != "" {
			c.key = key
		}
	}
}

// WithTransport sets the base transport every request is sent through, in place of
// http.DefaultTransport (or the transport dialing a unix socket address, which rt then has
// to dial itself). The client still wraps it with tracing, logging and its headers, so rt
// sees each request with those set. Use it to e.g. attach credentials per request from the
// request's context
func WithTransport(rt http.RoundTripper) Option {
	return func(c *DefaultClient) {
		if rt != nil {
			c.baseRT = rt
		}
	}
}

// WithBearerToken sets the token presented to the API server as a Bearer credential
// (Authorization: Bearer <token>) on every request. It takes precedence over WithAPIKey
func WithBearerToken(token string) Option {
	return func(c *DefaultClient) {
		if token != "" {
			c.token = token
		}
	}
}

// WithScheme sets the scheme for client requests. http is the default
func WithScheme(scheme string) Option {
	return func(c *DefaultClient) {
		if scheme != "" {
			c.scheme = scheme
		}
	}
}

// WithName sets the name which is included in the User-Agent header
func WithName(name string) Option {
	return func(c *DefaultClient) {
		if name != "" {
			c.name = fmt.Sprintf("%s/%s", name, version.Short())
		}
	}
}

const (
	// large request timeout to prevent unbounded request times. In practice, there will
	// likely be a request cancellation way before this timeout is hit
	defaultRequestTimeout = 30 * time.Minute
	defaultClientName     = "default-client"

	unixIdent = "unix"
)

// NewDefault creates a new default client that can be used for all calls to goProbe APIs
func NewDefault(addr string, opts ...Option) *DefaultClient {
	c := &DefaultClient{
		client:   http.DefaultClient,
		hostAddr: addr,
		name:     defaultClientName,
		retry:    true,
		retryIntervals: httpc.Intervals{
			// retry three times before giving up
			1 * time.Second, 2 * time.Second, 4 * time.Second,
		},
		scheme:  "http://",
		timeout: defaultRequestTimeout,
	}
	for _, opt := range opts {
		opt(c)
	}

	t := http.DefaultTransport

	// change transport to dial to the unix socket instead
	unixSocketFile := api.ExtractUnixSocket(addr)
	if unixSocketFile != "" {
		t = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, unixIdent, unixSocketFile)
			},
		}
		// also make sure to unset the address and modify the scheme so the calls to NewURL
		// don't append the filename of the unix socket
		c.hostAddr = ""
		c.scheme = c.scheme + unixIdent
	}
	if c.baseRT != nil {
		t = c.baseRT
	}

	// extract the scheme from the address if it is available
	scheme, addr := api.ExtractSchemeAddr(c.hostAddr)
	if scheme != "" {
		c.scheme = scheme
		c.hostAddr = addr
	}

	c.client = &http.Client{
		// trace propagation is enabled by default
		Transport: &transport{
			rt: otelhttp.NewTransport(
				t,
			),
			requestLogging: c.requestLogging,
			clientName:     c.name,
			authorization:  c.authorization(),
		},
	}
	return c
}

// authorization returns the value of the Authorization header sent with every request
func (c *DefaultClient) authorization() string {
	switch {
	case c.token != "":
		return "Bearer " + c.token
	case c.key != "":
		return "digest " + c.key
	}
	return ""
}

// Client returns the client's *http.Client
func (c *DefaultClient) Client() *http.Client {
	return c.client
}

type transport struct {
	rt             http.RoundTripper
	requestLogging bool
	clientName     string
	authorization  string
}

// RoundTrip implements the http.RoundTripper interface, adding tracing and
// logging (if enabled) to a client request. It sets the headers every request of the
// client carries, so that they are sent no matter how the request was built
func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()

	// a RoundTripper must not modify the request it is handed
	r = r.Clone(r.Context())
	r.Header.Set("User-Agent", t.clientName)
	r.Header.Set(server.RuntimeIDHeaderKey, info.RuntimeID())
	if t.authorization != "" && r.Header.Get("Authorization") == "" {
		r.Header.Set("Authorization", t.authorization)
	}

	// make request
	resp, err := t.rt.RoundTrip(r)
	duration := time.Since(start)

	if t.requestLogging {
		ctx := r.Context()

		logger := logging.FromContext(ctx).With("req", slog.GroupValue(
			slog.String("method", r.Method),
			slog.String("url", r.URL.String()),
			slog.String("user_agent", r.UserAgent()),
			slog.Duration("duration", duration),
		))
		// log trace ID if it is present
		sc := trace.SpanContextFromContext(ctx)
		if sc.HasTraceID() {
			logger = logger.With(slog.String("traceID", sc.TraceID().String()))
		}

		if err == nil {
			if resp != nil {
				logger = logger.With("resp", slog.GroupValue(
					slog.Int("status_code", resp.StatusCode),
				))
				switch {
				case 200 <= resp.StatusCode && resp.StatusCode < 300:
					logger.Info("completed request")
				case 300 <= resp.StatusCode && resp.StatusCode < 400:
					logger.Info("further action needed to complete request")
				default:
					logger.Error("server error returned")
				}
			} else {
				logger.Error("empty response")
			}
		} else {
			logger.Errorf("failed to send request: %v", err)
		}
	}
	return resp, err
}

// Modify activates retry behavior and timeout handling. Headers common to all requests,
// such as the credential, are set by the client's transport
func (c *DefaultClient) Modify(_ context.Context, req *httpc.Request) *httpc.Request {
	// retry any request that isn't 2xx
	if c.retry {
		req = req.RetryBackOff(c.retryIntervals).
			RetryBackOffErrFn(func(resp *http.Response, _ error) bool {
				// if the response is nil, we should try again definitely
				if resp == nil {
					return true
				}
				switch resp.StatusCode {
				case http.StatusBadGateway, http.StatusInternalServerError,
					http.StatusTooManyRequests:
					return true
				}
				return false
			})
	}
	if c.timeout > 0 {
		req = req.Timeout(c.timeout)
	}
	return req
}

// NewURL synthesizes a new URL for a given path depending on how the
// client was configured.
//
// Example:
//
//	http://localhost:8145/status
func (c *DefaultClient) NewURL(path string) string {
	return c.scheme + filepath.Join(c.hostAddr, path)
}
