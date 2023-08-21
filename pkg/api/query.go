package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/gin-gonic/gin"
)

func LogAndAbort(ctx context.Context, c *gin.Context, code int, err error) {
	logging.FromContext(ctx).Error(err)
	c.AbortWithError(code, err)
}

func RunQuery(caller, sourceData string, querier query.Runner, c *gin.Context) {
	ctx := c.Request.Context()

	// Initialize default query args
	var queryArgs = &query.Args{}

	// Parse args from request
	// if err := c.ShouldBind(queryArgs); err != nil {
	// 	LogAndAbort(ctx, c, http.StatusBadRequest, err)
	// 	return
	// }

	// the default format is json
	queryArgs.Format = "json"

	if queryArgs.Caller == "" {
		queryArgs.Caller = caller
	}

	// check if the statement can be created
	logger := logging.FromContext(ctx)

	logger.With("args", queryArgs).Info("running query")
	_, err := queryArgs.Prepare()
	if err != nil {
		LogAndAbort(ctx, c, http.StatusBadRequest, fmt.Errorf("failed to prepare query statement: %w", err))
		return
	}

	result, err := querier.Run(ctx, queryArgs)
	if err != nil {
		LogAndAbort(ctx, c, http.StatusBadRequest, fmt.Errorf("%s query failed: %w", sourceData, err))
		return
	}

	// serialize raw result if json is selected
	c.JSON(http.StatusOK, result)
}
