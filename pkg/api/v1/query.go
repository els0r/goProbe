package v1

import (
	"github.com/els0r/goProbe/pkg/api/json"

	"fmt"
	"net/http"

	"github.com/els0r/goProbe/pkg/query"
)

func (a *API) handleQuery(w http.ResponseWriter, r *http.Request) {
	// set up default options
	callerString := fmt.Sprintf("goProbe-API/%s", a.Version())
	opts := []query.Option{
		query.WithFormat("json"),
	}

	// create bare query arguments
	args := query.NewArgs("", "", opts...)

	// parse additional arguments from command line
	err := json.Parse(r, &args)
	if err != nil {
		status := http.StatusBadRequest
		errText := fmt.Sprintf("%s: failed to decode query arguments", http.StatusText(status))
		http.Error(w, errText, status)
	}

	// make sure that the caller variable is always the API
	args.Caller = callerString

	// prepare the query
	stmt, err := args.Prepare(w)
	if err != nil {
		status := http.StatusBadRequest
		errText := fmt.Sprintf("%s: failed to prepare query. Invalid arguments provided", http.StatusText(status))
		http.Error(w, errText, status)
	}

	// execute query
	err = stmt.Execute()
	if err != nil {
		status := http.StatusInternalServerError
		errText := fmt.Sprintf("%s: failed to execute query", http.StatusText(status))
		http.Error(w, errText, status)
	}
}
