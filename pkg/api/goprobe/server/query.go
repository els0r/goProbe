package server

import (
	"fmt"

	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/goDB/engine"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/gin-gonic/gin"
)

func (server *Server) postQuery(c *gin.Context) {
	api.RunQuery(
		fmt.Sprintf("goProbe/%s", version.Short()),
		"local DB",
		engine.NewQueryRunner(server.dbPath),
		c,
	)
}
