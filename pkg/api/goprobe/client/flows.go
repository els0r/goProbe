package client

import (
	"context"
	"fmt"
	"strings"

	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/fako1024/httpc"
)

// GetActiveFlows returns the active flows on the interface(s) captured by the running goProbe instance
func (c *Client) GetActiveFlows(ctx context.Context, ifaces ...string) (map[string]capturetypes.FlowInfos, error) {
	var res = new(gpapi.FlowsResponse)

	url := c.NewURL(gpapi.FlowsRoute)
	if len(ifaces) == 1 {
		url += "/" + ifaces[0]
	}

	req := c.Modify(ctx,
		httpc.NewWithClient("GET", url, c.Client()).
			ParseJSON(res),
	)
	if len(ifaces) > 1 {
		req = req.QueryParams(httpc.Params{
			gpapi.IfacesQueryParam: strings.Join(ifaces, ","),
		})
	}
	err := req.RunWithContext(ctx)
	if err != nil {
		if res.Error != "" {
			err = fmt.Errorf("%d: %s", res.StatusCode, res.Error)
		}
		return nil, err
	}
	return res.Flows, nil
}
