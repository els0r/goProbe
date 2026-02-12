package results

import "context"

// PostProcessor is a function that post-processes a query result
type PostProcessor func(context.Context, *Result) error
