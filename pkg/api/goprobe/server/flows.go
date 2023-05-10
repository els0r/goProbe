package server

import (
	"net/http"
	"net/url"
	"strings"

	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/gin-gonic/gin"
)

func (server *Server) getFlows(c *gin.Context) {
	iface := c.Param(ifaceKey)
	ifaces := c.Request.URL.Query().Get(gpapi.IfacesQueryParam)

	resp := &gpapi.FlowsResponse{}
	resp.StatusCode = http.StatusOK

	var err error
	ifaces, err = url.QueryUnescape(ifaces)
	if err != nil {
		resp.StatusCode = http.StatusBadRequest
		resp.Error = err.Error()

		c.AbortWithStatusJSON(resp.StatusCode, resp)
		return
	}

	if iface != "" {
		resp.Flows = server.captureManager.ActiveFlows(iface)
	} else {
		if ifaces != "" {
			resp.Flows = server.captureManager.ActiveFlows(strings.Split(ifaces, ",")...)
		} else {
			// otherwise, fetch all
			resp.Flows = server.captureManager.ActiveFlows()
		}
	}

	if len(resp.Flows) == 0 {
		resp.StatusCode = http.StatusNoContent
	}

	c.JSON(resp.StatusCode, resp)
	return
}
