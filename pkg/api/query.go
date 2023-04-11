package api

import (
	"fmt"
	"net/http"

	"github.com/els0r/goProbe/pkg/logging"
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

	// the default format is json
	queryArgs.Format = "json"

	if queryArgs.Caller == "" {
		queryArgs.Caller = caller
	}

	// check if the statement can be created
	logger := logging.WithContext(ctx).With("args", queryArgs)

	logger.Info("running query")
	_, err = queryArgs.Prepare()
	if err != nil {
		err = fmt.Errorf("failed to prepare query statement: %v", err)
		logger.Error(err)
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	result, err := querier.Run(ctx, queryArgs)
	if err != nil {
		err = fmt.Errorf("%s query failed: %w", sourceData, err)
		logger.Error(err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// serialize raw result if json is selected
	c.JSON(http.StatusOK, result)
}
