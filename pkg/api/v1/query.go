package v1

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/els0r/goProbe/pkg/api/json"
	"github.com/els0r/goProbe/pkg/goDB/engine"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
)

// handleQuery can be used to query the flow database that goPorbe writes
// to disk. It is the equivalent of calling the binary `goQuery` on
// the machine running goProbe.
func (a *API) handleQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// set up default options
	callerString := fmt.Sprintf("goProbe-API/%s", a.Version())
	opts := []query.Option{
		query.WithFormat("json"),
	}

	// create bare query arguments
	args := query.NewArgs("", "", opts...)

	// parse additional arguments from command line
	if err := json.Parse(r, &args); err != nil {
		a.errorHandler.Handle(ctx, w, http.StatusBadRequest, err, "failed to decode query arguments")
		return
	}

	// make sure that the caller variable is always the API
	args.Caller = callerString

	// do not allow the caller to set more than the default
	// maximum memory use. The API should not be an entrypoint
	// to exhaust host resources
	if args.MaxMemPct > query.DefaultMaxMemPct {
		args.MaxMemPct = query.DefaultMaxMemPct
	}

	// prepare the query
	stmt, err := args.Prepare(w)
	if err != nil {
		a.errorHandler.Handle(ctx, w, http.StatusBadRequest, err, "failed to prepare query. Invalid arguments provided")
		return
	}

	// execute query
	res, err := engine.NewQueryRunner().Run(ctx, stmt)
	if err != nil {
		a.errorHandler.Handle(ctx, w, http.StatusInternalServerError, err, "failed to execute query")
		return
	}

	result := res[0]
	if stmt.Format == "json" {
		err = jsoniter.NewEncoder(w).Encode(result)
		if err != nil {
			a.errorHandler.Handle(ctx, w, http.StatusInternalServerError, err, "failed to JSON serialize results")
			return
		}
		return
	}

	// when running against a local goDB, there should be exactly one result
	result = res[0]

	statusCode := http.StatusInternalServerError
	switch result.Status {
	case types.StatusOK:
		err = stmt.Print(ctx, result)
		if err != nil {
			a.errorHandler.Handle(ctx, w, http.StatusInternalServerError, err, "failed to write results")
			return
		}
		return
	case types.StatusEmpty:
		statusCode = http.StatusNoContent
	}

	a.errorHandler.Handle(ctx, w, statusCode, errors.New(result.StatusMessage), string(result.Status))
	return
}
