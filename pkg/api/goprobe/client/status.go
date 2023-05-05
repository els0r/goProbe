package client

import (
	"context"
	"fmt"
	"net/http"

	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/fako1024/httpc"
)

// GetInterfaceStatus returns the interface capture stats from the running goProbe instance
func (c *Client) GetInterfaceStatus(ctx context.Context, ifaces ...string) (*gpapi.StatusResponse, error) {
	var res = new(gpapi.StatusResponse)

	url := c.NewURL(gpapi.StatusRoute)
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
