package client

import (
	"context"
	"fmt"
	"net/http"

	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/fako1024/httpc"
)

// GetActiveFlows returns the active flows on the interface(s) captured by the running goProbe instance
func (c *Client) GetActiveFlows(ctx context.Context, ifaces ...string) (*gpapi.FlowsResponse, error) {
	var res = new(gpapi.FlowsResponse)

	url := c.NewURL(gpapi.FlowsRoute)
	if len(ifaces) == 1 {
		url += "/" + ifaces[0]
	}

	req := c.Modify(ctx,
		httpc.NewWithClient("GET", url, c.Client()).
			ParseJSON(res),
	)
	err := req.RunWithContext(ctx)
	if err != nil {
		return nil, err
	}

	switch res.StatusCode {
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("%d: %s", res.StatusCode, res.Error)
	}

	return res, nil
}
