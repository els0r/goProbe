package server

import (
	"net/http"

	gpapi "github.com/els0r/goProbe/pkg/api/goprobe"
	"github.com/gin-gonic/gin"
)

func (server *Server) getStatus(c *gin.Context) {
	iface := c.Param(ifaceKey)

	resp := &gpapi.StatusResponse{}
	resp.StatusCode = http.StatusOK
	resp.LastWriteout = server.writeoutHandler.LastRotation

	if iface != "" {
		statuses, err := server.captureManager.Status(iface)
		if err != nil {
			resp.Error = err.Error()
			resp.StatusCode = http.StatusInternalServerError

			c.AbortWithStatusJSON(resp.StatusCode, resp)
			return
		}

		resp.Statuses = statuses

		c.JSON(resp.StatusCode, resp)
		return
	}

	// otherwise, fetch all
	statuses := server.captureManager.StatusAll()
	if len(statuses) == 0 {
		resp.StatusCode = http.StatusNoContent

		c.JSON(resp.StatusCode, resp)
		return
	}
	resp.Statuses = statuses

	c.JSON(resp.StatusCode, resp)
	return
}
