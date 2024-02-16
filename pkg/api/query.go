package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/telemetry/logging"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	jsoniter "github.com/json-iterator/go"
)

// LogAndAbort logs an error and the aborts further processing
func LogAndAbort(ctx context.Context, c *gin.Context, code int, err error) {
	logging.FromContext(ctx).Error(c.AbortWithError(code, err))
}

// RunQuery executes the query and returns its result
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
		LogAndAbort(ctx, c, http.StatusInternalServerError, fmt.Errorf("%s query failed: %w", sourceData, err))
		return
	}

	// serialize raw result if json is selected
	c.JSON(http.StatusOK, result)
}

// ValidationHandler returns the query args validation handler
func ValidationHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		queryArgs := new(query.Args)

		// Attempt to parse args from request JSON body
		if err := jsoniter.NewDecoder(c.Request.Body).Decode(queryArgs); err != nil {

			// If that failed, attempt to bind the URL form data
			if err = binding.Form.Bind(c.Request, queryArgs); err != nil {
				LogAndAbort(ctx, c, http.StatusBadRequest, err)
				return
			}
		}

		logger := logging.FromContext(ctx).With("args", queryArgs)

		logger.Debug("validating args")
		_, err := queryArgs.Prepare()
		if err != nil {
			vr := &ValidationResponse{StatusCode: http.StatusBadRequest}

			if !errors.As(err, &vr.ArgsError) {
				LogAndAbort(ctx, c, http.StatusInternalServerError, err)
				return
			}

			logger.With("error", vr.ArgsError).Error("invalid query args")

			c.JSON(http.StatusBadRequest, vr)
			return
		}

		c.JSON(http.StatusNoContent, "")
	}
}

// ValidationResponse stores the response to a validation query
type ValidationResponse struct {
	StatusCode int `json:"status_code"` // StatusCode: stores the HTTP status code of the response. Example: 200
	*query.ArgsError
}
