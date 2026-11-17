// Package engine executes query.Args against a local goDB and returns results.Result.
//
// RunStreaming delivers partial results through an sse.Sender (api.SSEQueryRunner).
//
// Runner behaviour (concurrency limit, stats callbacks) is set through RunnerOption.
package engine
