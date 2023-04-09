package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/els0r/goProbe/pkg/global-query/routes"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/fako1024/httpc"
)

type Client struct {
	scheme   string
	hostAddr string

	client *http.Client

	timeout time.Duration
}

func (c *Client) newURL(path string) string {
	return fmt.Sprintf("%s://%s/%s", c.scheme, c.hostAddr, path)
}

func (c *Client) Query(ctx context.Context, args *query.Args) (*results.Result, error) {
	// use a copy of the arguments, since some fields are modified by the client
	queryArgs := *args
	// whatever happens, the results are expected to be returned in json
	queryArgs.Format = "json"

	var res = new(results.Result)

	req := httpc.NewWithClient("POST", c.newURL(routes.Query), c.client).
		EncodeJSON(queryArgs).
		Timeout(c.timeout).
		RetryBackOffErrFn(func(resp *http.Response, _ error) bool {
			return resp.StatusCode == http.StatusTooManyRequests
		}).
		ParseJSON(res)

	err := req.RunWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run request: %w", err)
	}

	return res, nil
}
