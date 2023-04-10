package api

import (
	"fmt"
	"net/http"

	"github.com/els0r/goProbe/pkg/query"
	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
)

func RunQuery(caller, sourceData string, querier query.Runner, c *gin.Context) {
	ctx := c.Request.Context()

	// parse query args from request
	var queryArgs = new(query.Args)
	err := jsoniter.NewDecoder(c.Request.Body).Decode(queryArgs)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// make sure that the caller variable is always the API
	queryArgs.Caller = caller

	// the default format is json
	queryArgs.Format = "json"

	// check if the statement can be created
	_, err = queryArgs.Prepare()
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("failed to prepare query statement: %v", err))
		return
	}

	result, err := querier.Run(ctx, queryArgs)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("%s query failed: %w", sourceData, err))
		return
	}

	// serialize raw result if json is selected
	c.JSON(http.StatusOK, result)
}
