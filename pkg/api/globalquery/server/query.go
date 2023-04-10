package server

import (
	"fmt"

	"github.com/els0r/goProbe/cmd/global-query/pkg/distributed"
	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/gin-gonic/gin"
)

// RegisterQueryHandler hooks up the distributed query endpoint to an existing gin engine. It is meant for third-party
// APIs as a means to integrate query capabilities
func RegisterQueryHandler(engine *gin.Engine, route string, resolver hosts.Resolver, querier distributed.Querier) {
	engine.POST(route, func(c *gin.Context) {
		api.RunQuery(
			fmt.Sprintf("global-query/%s", version.Short()),
			"distributed",
			distributed.NewQueryRunner(resolver, querier),
			c,
		)
	})
}
