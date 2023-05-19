package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/gin-gonic/gin"
)

func (server *Server) getConfig(c *gin.Context) {
	iface := c.Param(ifaceKey)
	ifaces := c.Request.URL.Query().Get(gpapi.IfacesQueryParam)

	resp := &gpapi.ConfigResponse{}
	resp.StatusCode = http.StatusOK

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
		resp.Ifaces = server.captureManager.Config(ctx, iface)
	} else {
		if ifaces != "" {
			// fetch all specified
			resp.Ifaces = server.captureManager.Config(ctx, strings.Split(ifaces, ",")...)
		} else {
			// otherwise, fetch all
			resp.Ifaces = server.captureManager.Config(ctx)
		}
	}

	if len(resp.Ifaces) == 0 {
		resp.StatusCode = http.StatusNoContent
	}

	c.JSON(resp.StatusCode, resp)
	return
}

func (server *Server) putConfig(c *gin.Context) {
	resp := &gpapi.ConfigUpdateResponse{}
	resp.StatusCode = http.StatusOK

	// de-serialize the configuration
	var ifaceConfigs config.Ifaces

	err := c.BindJSON(&ifaceConfigs)
	if err != nil {
		resp.StatusCode = http.StatusBadRequest
		resp.Error = err.Error()

		c.AbortWithStatusJSON(resp.StatusCode, resp)
		return
	}

	// update the captures
	ctx := c.Request.Context()

	resp.Enabled, resp.Updated, resp.Disabled, err = server.captureManager.Update(ctx, ifaceConfigs)
	if err != nil {
		resp.StatusCode = http.StatusBadRequest
		resp.Error = err.Error()

		c.AbortWithStatusJSON(resp.StatusCode, resp)
	}

	c.JSON(resp.StatusCode, resp)
	return
}
