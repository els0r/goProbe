package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/fako1024/httpc"
)

// GetInterfaceStatus returns the interface capture stats from the running goProbe instance
func (c *Client) GetInterfaceStatus(ctx context.Context, ifaces ...string) (statuses map[string]capturetypes.InterfaceStatus, lastWriteout time.Time, err error) {
	var res = new(gpapi.StatusResponse)

	url := c.NewURL(gpapi.StatusRoute)
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
		return nil, lastWriteout, err
	}

	return res.Statuses, res.LastWriteout, nil
}
