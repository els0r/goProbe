package v1

import (
	"fmt"
	"net/http"

	"github.com/els0r/goProbe/pkg/api/json"
	"github.com/els0r/goProbe/pkg/query"
	jsoniter "github.com/json-iterator/go"
)

// handleQuery can be used to query the flow database that goPorbe writes
// to disk. It is the equivalent of calling the binary `goQuery` on
// the machine running goProbe.
func (a *API) handleQuery(w http.ResponseWriter, r *http.Request) {
	// set up default options
	callerString := fmt.Sprintf("goProbe-API/%s", a.Version())
	opts := []query.Option{
		query.WithFormat("json"),
	}

	// create bare query arguments
	args := query.NewArgs("", "", opts...)

	// parse additional arguments from command line
	if err := json.Parse(r, &args); err != nil {
		a.errorHandler.Handle(w, http.StatusBadRequest, err, "failed to decode query arguments")
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
		a.errorHandler.Handle(w, http.StatusBadRequest, err, "failed to prepare query. Invalid arguments provided")
		return
	}

	// execute query
	result, err := stmt.Execute(r.Context())
	if err != nil {
		a.errorHandler.Handle(w, http.StatusInternalServerError, err, "failed to execute query")
		return
	}

	if args.Format == "json" {
		err = jsoniter.NewEncoder(w).Encode(result)
		if err != nil {
			a.errorHandler.Handle(w, http.StatusInternalServerError, err, "failed to JSON serialize results")
			return
		}
		return
	}

	err = stmt.Print(r.Context(), result)
	if err != nil {
		a.errorHandler.Handle(w, http.StatusInternalServerError, err, "failed to write results")
		return
	}
}
