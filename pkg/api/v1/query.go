package v1

import (
	"fmt"
	"net/http"

	"github.com/els0r/goProbe/pkg/api/json"
	"github.com/els0r/goProbe/pkg/goDB/engine"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
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

	emptyResReturn := func(stmt *query.Statement) {
		if stmt.External || stmt.Format == "json" {
			msg := results.ErrorMsgExternal{Status: types.StatusEmpty, Message: results.ErrorNoResults.Error()}
			jsoniter.NewEncoder(w).Encode(msg)
		}
	}

	// execute query
	res, err := engine.NewQueryRunner().Run(ctx, stmt)
	if err != nil {
		a.errorHandler.Handle(ctx, w, http.StatusInternalServerError, err, "failed to execute query")
		return
	}
	// empty results should be handled here exclusively
	if len(res) == 0 {
		emptyResReturn(stmt)
		return
	} else if len(res) > 1 {
		a.errorHandler.Handle(ctx, w, http.StatusInternalServerError, err, "unexpected number of results encountered")
		return
	}

	result := res[0]
	if args.Format == "json" {
		// handle empty results only for the external case. Otherwise,
		// return the entire result data structure with empty "rows"
		if len(result.Rows) == 0 && stmt.External {
			msg := results.ErrorMsgExternal{Status: types.StatusEmpty, Message: results.ErrorNoResults.Error()}
			jsoniter.NewEncoder(w).Encode(msg)
			return
		}
		err = jsoniter.NewEncoder(w).Encode(result)
		if err != nil {
			a.errorHandler.Handle(ctx, w, http.StatusInternalServerError, err, "failed to JSON serialize results")
			return
		}
		return
	}

	err = stmt.Print(ctx, &result)
	if err != nil {
		a.errorHandler.Handle(ctx, w, http.StatusInternalServerError, err, "failed to write results")
		return
	}
}
