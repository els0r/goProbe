package server

import (
	"net/http"
	"net/url"
	"strings"

	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/gin-gonic/gin"
)

func (server *Server) getStatus(c *gin.Context) {
	iface := c.Param(ifaceKey)
	ifaces := c.Request.URL.Query().Get(gpapi.IfacesQueryParam)

	resp := &gpapi.StatusResponse{}
	resp.StatusCode = http.StatusOK
	resp.LastWriteout = server.captureManager.LastRotation()

	var err error
	ifaces, err = url.QueryUnescape(ifaces)
	if err != nil {
		resp.StatusCode = http.StatusBadRequest
		resp.Error = err.Error()

		c.AbortWithStatusJSON(resp.StatusCode, resp)
		return
	}

	ctx := c.Request.Context()

	if iface != "" {
		resp.Statuses = server.captureManager.Status(ctx, iface)
	} else {
		if ifaces != "" {
			// fetch all specified
			resp.Statuses = server.captureManager.Status(ctx, strings.Split(ifaces, ",")...)
		} else {
			// otherwise, fetch all
			resp.Statuses = server.captureManager.Status(ctx)
		}
	}

	if len(resp.Statuses) == 0 {
		resp.StatusCode = http.StatusNoContent
	}

	c.JSON(resp.StatusCode, resp)
	return
}
