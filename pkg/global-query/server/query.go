package server

import (
	"fmt"
	"net/http"

	"github.com/els0r/goProbe/cmd/global-query/pkg/distributed"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
)

const (
	queryHostname = "hostname"
	queryHostID   = "hostid"
)

func (server *Server) postQuery(c *gin.Context) {
	ctx := c.Request.Context()

	// parse query args from request
	var queryArgs = new(query.Args)
	err := jsoniter.NewDecoder(c.Request.Body).Decode(queryArgs)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// check if the statement can be created
	_, err = queryArgs.Prepare()
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("failed to prepare query statement: %v", err))
		return
	}

	finalResult, err := distributed.NewQuerier(server.hostListResolver, server.querier).Run(ctx, queryArgs)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("distributed query failed: %w", err))
		return
	}

	// serialize raw result if json is selected
	c.JSON(http.StatusOK, finalResult)
	return
}
