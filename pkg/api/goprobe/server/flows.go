package server

import (
	"net/http"

	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/gin-gonic/gin"
)

func (server *Server) getFlows(c *gin.Context) {
	iface := c.Param(ifaceKey)

	resp := &gpapi.FlowsResponse{}
	resp.StatusCode = http.StatusOK

	// TODO: make sure that the all case is only hit when no iface is specified
	// --> ?ifaces=eth0,eth1
	if iface != "" {
		resp.Flows = server.captureManager.ActiveFlows(iface)
	} else {
		// otherwise, fetch all
		resp.Flows = server.captureManager.ActiveFlows()
	}

	if len(resp.Flows) == 0 {
		resp.StatusCode = http.StatusNoContent
	}

	c.JSON(resp.StatusCode, resp)
	return
}
