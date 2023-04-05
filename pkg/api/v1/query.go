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

// handleQuery can be used to query the flow database that goProbe writes
// to disk. It is the equivalent of calling the binary `goQuery` on
// the machine running goProbe.
func (a *API) handleQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// set up default options
	callerString := fmt.Sprintf("goProbe-API/%s", a.Version())

	// the default format is json
	var stmt = new(query.Statement)
	stmt.Format = "json"
	stmt.Output = w

	// make sure that the caller variable is always the API
	stmt.Caller = callerString

	// parse additional arguments from command line
	if err := json.Parse(r, stmt); err != nil {
		a.errorHandler.Handle(ctx, w, http.StatusBadRequest, err, "failed to decode query statement")
		return
	}

	// do not allow the caller to set more than the default
	// maximum memory use. The API should not be an entrypoint
	// to exhaust host resources
	if stmt.MaxMemPct > query.DefaultMaxMemPct {
		stmt.MaxMemPct = query.DefaultMaxMemPct
	}

	// execute query
	res, err := engine.NewQueryRunner().Run(ctx, stmt)
	if err != nil {
		a.errorHandler.Handle(ctx, w, http.StatusInternalServerError, err, "failed to execute query")
		return
	}

	result := res
	if stmt.Format == "json" {
		err = jsoniter.NewEncoder(w).Encode(result)
		if err != nil {
			a.errorHandler.Handle(ctx, w, http.StatusInternalServerError, err, "failed to JSON serialize results")
			return
		}
		return
	}

	// when running against a local goDB, there should be exactly one result
	result = res

	statusCode := http.StatusInternalServerError
	switch result.Status.Code {
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

	a.errorHandler.Handle(ctx, w, statusCode, errors.New(result.Status.Message), string(result.Status.Code))
	return
}
