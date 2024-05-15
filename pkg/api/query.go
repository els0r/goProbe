package api

import (
	"context"

	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/telemetry/logging"
)

func getBodyQueryRunnerHandler(caller string, querier query.Runner) func(context.Context, *ArgsInput) (*QueryResultOutput, error) {
	return func(ctx context.Context, input *ArgsInput) (*QueryResultOutput, error) {
		output := &QueryResultOutput{}

		res, err := runQuery(ctx, caller, input.Body, querier)
		if err != nil {
			return nil, err
		}
		output.Body = res

		return output, nil
	}
}

func runQuery(ctx context.Context, caller string, args *query.Args, querier query.Runner) (*results.Result, error) {
	// make sure all defaults are available if they weren't set explicitly
	args.SetDefaults()

	// Set default format for an API query is JSON
	args.Format = "json"
	if args.Caller == "" {
		args.Caller = caller
	}

	logger := logging.FromContext(ctx)

	// Check if the statement can be created
	logger.With("args", args).Info("running query")
	_, err := args.Prepare()
	if err != nil {
		return nil, err
	}

	result, err := querier.Run(ctx, args)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// getBodyValidationHandler returns the query args validation handler
func getBodyValidationHandler() func(context.Context, *ArgsInput) (*struct{}, error) {
	return func(ctx context.Context, input *ArgsInput) (*struct{}, error) {
		args := input.Body

		logger := logging.FromContext(ctx).With("args", args)
		logger.Debug("validating args from body")

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

// getParamsValidationHandler returns the query args validation handler
func getParamsValidationHandler() func(context.Context, *ArgsParamsInput) (*struct{}, error) {
	return func(ctx context.Context, input *ArgsParamsInput) (*struct{}, error) {
		args := input.Args
		args.DNSResolution = input.DNSResolution

		logger := logging.FromContext(ctx).With("args", args)
		logger.Debug("validating args from query parameters")

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
