package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (server *Server) postConfig(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, nil)
}
