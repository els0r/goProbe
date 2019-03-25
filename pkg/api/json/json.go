package json

import (
	"net/http"

	// high-performance drop-in for json
	"github.com/json-iterator/go"
)

var j = jsoniter.ConfigCompatibleWithStandardLibrary

// Response is a wrapper around the normal encoder in order to
// properly set the content-type header
func Response(w http.ResponseWriter, val interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return j.NewEncoder(w).Encode(val)
}
