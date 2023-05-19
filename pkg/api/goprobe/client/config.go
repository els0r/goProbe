package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/fako1024/httpc"
)

func (c *Client) GetInterfaceConfig(ctx context.Context, ifaces ...string) (ifaceConfigs config.Ifaces, err error) {
	var res = new(gpapi.ConfigResponse)

	url := c.NewURL(gpapi.ConfigRoute)
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
	err = req.RunWithContext(ctx)
	if err != nil {
		if res.Error != "" {
			err = fmt.Errorf("%d: %s", res.StatusCode, res.Error)
		}
		return nil, err
	}
	return res.Ifaces, nil
}

func (c *Client) UpdateInterfaceConfigs(ctx context.Context, ifaceConfigs config.Ifaces) (updated, disabled []string, err error) {
	var res = new(gpapi.ConfigUpdateResponse)

	url := c.NewURL(gpapi.ConfigRoute)

	req := c.Modify(ctx,
		httpc.NewWithClient("PUT", url, c.Client()).
			EncodeJSON(ifaceConfigs).
			ParseJSON(res),
	)
	err = req.RunWithContext(ctx)
	if err != nil {
		if res.Error != "" {
			err = fmt.Errorf("%d: %s", res.StatusCode, res.Error)
		}
		return
	}
	return res.Disabled, res.Updated, nil
}
