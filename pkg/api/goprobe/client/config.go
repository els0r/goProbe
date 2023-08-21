package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/fako1024/httpc"
)

// GetInterfaceConfig returns goprobe's runtime configuration for the queried interfaces. If ifaces
// is empty or omitted, the runtime configuration for all interfaces is returned
func (c *Client) GetInterfaceConfig(ctx context.Context, ifaces ...string) (ifaceConfigs config.Ifaces, err error) {
	var res = new(gpapi.ConfigResponse)

	url := c.NewURL(addIfaceToPath(gpapi.ConfigRoute, ifaces...))

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

// UpdateInterfaceConfigs updates goprobe's runtime configuration for the provided interfaces
func (c *Client) UpdateInterfaceConfigs(ctx context.Context, ifaceConfigs config.Ifaces) (enabled, updated, disabled []string, err error) {
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
	return res.Enabled, res.Updated, res.Disabled, nil
}

// ReloadConfig reads / updates goprobe's runtime configuration with the one from disk
func (c *Client) ReloadConfig(ctx context.Context) error {

	url := c.NewURL(gpapi.ConfigRoute + gpapi.ConfigReloadRoute)

	req := c.Modify(ctx,
		httpc.NewWithClient("POST", url, c.Client()),
	)

	if err := req.RunWithContext(ctx); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}
