package server

import (
	"fmt"
	"strconv"

	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/goDB/engine"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/gin-gonic/gin"
)

const liveQueryKey = "live"

func (server *Server) postQuery(c *gin.Context) {

	// Ascertain if a live query was requested or not (the liveQueryKey is not part
	// of the common query args since it is only supported via this API)
	var runner *engine.QueryRunner
	if isLiveQuery(c) {
		runner = engine.NewQueryRunnerWithLiveData(server.dbPath, server.captureManager)
	} else {
		runner = engine.NewQueryRunner(server.dbPath)
	}

	api.RunQuery(
		fmt.Sprintf("goProbe/%s", version.Short()),
		"local DB",
		runner,
		c,
	)
}

func isLiveQuery(c *gin.Context) bool {
	isLive, err := strconv.ParseBool(c.Query(liveQueryKey))
	if err != nil {
		return false
	}

	return isLive
}
