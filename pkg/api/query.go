package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	jsoniter "github.com/json-iterator/go"
)

func LogAndAbort(ctx context.Context, c *gin.Context, code int, err error) {
	logging.FromContext(ctx).Error(err)
	c.AbortWithError(code, err)
}

func RunQuery(caller, sourceData string, querier query.Runner, c *gin.Context) {
	ctx := c.Request.Context()

	// Initialize default query args
	var queryArgs = query.DefaultArgs()

	// Attempt to parse args from request JSON body
	if err := jsoniter.NewDecoder(c.Request.Body).Decode(queryArgs); err != nil {

		// If that failed, attempt to bind the URL form data
		if err = binding.Form.Bind(c.Request, queryArgs); err != nil {
			LogAndAbort(ctx, c, http.StatusBadRequest, err)
			return
		}
	}

	// Set default format for an API query is JSON
	queryArgs.Format = "json"
	if queryArgs.Caller == "" {
		queryArgs.Caller = caller
	}

	logger := logging.FromContext(ctx)

	// Check if the statement can be created
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
