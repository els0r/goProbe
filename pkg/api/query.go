package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/telemetry/logging"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	jsoniter "github.com/json-iterator/go"
)

var queryTags = []string{"Query"}

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

func GetValidationOperation() huma.Operation {
	return huma.Operation{
		OperationID: "query-validate",
		Method:      http.MethodPost,
		Path:        ValidationRoute,
		Summary:     "Validate query parameters",
		Description: "Validates query parameters for (1) integrity (2) attempting to prepare a query statement from them",
		Tags:        queryTags,
	}
}

// ValidationInput stores the query args to be validated
type ValidationInput struct {
	Body struct {
		*query.Args
	}
}

// GetValidationHandler returns the query args validation handler
func GetValidationHandler() func(context.Context, *ValidationInput) (*struct{}, error) {
	return func(ctx context.Context, input *ValidationInput) (*struct{}, error) {
		args := input.Body.Args

		logger := logging.FromContext(ctx).With("args", args)
		logger.Debug("validating args")

		_, err := args.Prepare()
		if err != nil {
			logger.With("error", err).Error("invalid query args")
			// if it's a validation error 422 is returned automatically
			return nil, err
		}

		// 204 No Content is added since no data is returned and no error is returned
		return nil, nil
	}
}
