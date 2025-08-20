package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/telemetry/logging"
)

var (
	// ErrTooManyConcurrentRequest denotes that the number of concurrent queries
	// has been exhausted
	ErrTooManyConcurrentRequest = errors.New("too many concurrent requests")
)

// OnResult is a generic handler / function that sends a result via an SSE sender
// to the client(s)
func OnResult(res *results.Result, send sse.Sender) error {
	if res == nil {
		return nil
	}

	return send.Data(&PartialResult{res})
}

// OnKeepalive is a generic handler / function that sends a keepalive signal via an SSE sender
// to the client(s)
func OnKeepalive(send sse.Sender) error {
	return send.Data(&Keepalive{})
}

// SSEQueryRunner defines any query runner that supports partial results / SSE
type SSEQueryRunner interface {
	// RunStreaming takes a query statement, executes the underlying query and returns the result(s)
	// while sending partial results to the sse.Sender
	RunStreaming(ctx context.Context, args *query.Args, send sse.Sender) (*results.Result, error)

	query.Runner
}

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

func getSSEBodyQueryRunnerHandler(caller string, querier SSEQueryRunner) func(context.Context, *ArgsInput, sse.Sender) {
	return func(ctx context.Context, input *ArgsInput, send sse.Sender) {
		res, err := runQuerySSE(ctx, caller, input.Body, querier, send)
		if err != nil {
			_ = send.Data(query.NewDetailError(http.StatusInternalServerError, err))
			return
		}

		_ = send.Data(&FinalResult{res})
	}
}

func runQuery(ctx context.Context, caller string, args *query.Args, querier query.Runner) (*results.Result, error) {
	if err := prepareArgs(ctx, caller, args); err != nil {
		return nil, err
	}

	return querier.Run(ctx, args)
}

func runQuerySSE(ctx context.Context, caller string, args *query.Args, querier SSEQueryRunner, send sse.Sender) (*results.Result, error) {
	if err := prepareArgs(ctx, caller, args); err != nil {
		return nil, err
	}

	return querier.RunStreaming(ctx, args, send)
}

type validationFunc func(*query.Args) error

// getBodyValidationHandler returns the query args validation handler
func getBodyValidationHandler(extraValidation ...validationFunc) func(context.Context, *ArgsInput) (*struct{}, error) {
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

		if len(extraValidation) == 0 {
			// 204 No Content is added since no data is returned and no error is returned
			return nil, nil
		}

		for _, validate := range extraValidation {
			if err := validate(args); err != nil {
				logger.With("error", err).Error("extra validation: invalid query args")
				return nil, err
			}
		}

		return nil, nil
	}
}

// getParamsValidationHandler returns the query args validation handler
func getParamsValidationHandler(extraValidation ...validationFunc) func(context.Context, *ArgsParamsInput) (*struct{}, error) {
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

		if len(extraValidation) == 0 {
			// 204 No Content is added since no data is returned and no error is returned
			return nil, nil
		}

		for _, validate := range extraValidation {
			if err := validate(&args); err != nil {
				logger.With("error", err).Error("extra validation: invalid query args")
				return nil, err
			}
		}

		return nil, nil
	}
}

func prepareArgs(ctx context.Context, caller string, args *query.Args) error {
	// make sure all defaults are available if they weren't set explicitly
	args.SetDefaults()

	// Set default format for an API query is JSON
	args.Format = types.FormatJSON
	if args.Caller == "" {
		args.Caller = caller
	}

	logger := logging.FromContext(ctx)

	// Check if the statement can be created
	logger.With("args", args).Info("running query")

	_, err := args.Prepare()
	return err
}
