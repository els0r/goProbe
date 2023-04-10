package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/els0r/goProbe/pkg/logging"
	"github.com/fako1024/httpc"
	"golang.org/x/exp/slog"
)

type DefaultClient struct {
	client  *http.Client
	timeout time.Duration

	scheme   string
	hostAddr string
	key      string

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

const defaultRequestTimeout = 30 * time.Second

// NewDefault creates a new default client that can be used for all calls to goProbe APIs
func NewDefault(addr string, opts ...Option) *DefaultClient {
	c := &DefaultClient{
		client:   http.DefaultClient,
		scheme:   "http",
		hostAddr: addr,
		timeout:  defaultRequestTimeout,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.requestLogging {
		c.client.Transport = &transport{
			rt:             http.DefaultTransport,
			requestLogging: true,
		}
	}
	return c
}

func (c *DefaultClient) Client() *http.Client {
	return c.client
}

type transport struct {
	rt             http.RoundTripper
	requestLogging bool
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()

	// make request
	resp, err := t.rt.RoundTrip(r)
	duration := time.Since(start)

	if t.requestLogging {
		logger := logging.WithContext(r.Context()).With("req", slog.GroupValue(
			slog.String("method", r.Method),
			slog.String("url", r.URL.String()),
			slog.Duration("duration", duration),
		))
		if resp != nil {
			logger = logger.With("resp", slog.GroupValue(
				slog.Int("status_code", resp.StatusCode),
			))
		}
		if err == nil && 200 <= resp.StatusCode && resp.StatusCode < 300 {
			logger.Info("successful request")
		} else {
			if err != nil {
				logger.Errorf("failed request: %v", err)
			} else {
				logger.Errorf("failed request")
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
