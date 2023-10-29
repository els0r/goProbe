package query

import (
	"sort"
	"time"

	"github.com/els0r/goProbe/pkg/defaults"
	"github.com/els0r/goProbe/pkg/results"
)

// Defaults for query arguments
var (
	DefaultDBPath         = defaults.DBPath
	DefaultFormat         = "txt"
	DefaultIn             = false
	DefaultMaxMemPct      = 60
	DefaultNumResults     = uint64(1000)
	DefaultOut            = false
	DefaultResolveRows    = 25
	DefaultResolveTimeout = 1 * time.Second
	DefaultQueryTimeout   = defaults.QueryTimeout
	DefaultSortBy         = "bytes"
)

// PermittedFormats stores all supported output formats
var permittedFormats = map[string]struct{}{
	"txt":  {},
	"json": {},
	"csv":  {},
}

var (
	permittedFormatsSlice = []string{}
	permittedSortBySlice  = []string{}
)

func init() {
	for format := range permittedFormats {
		permittedFormatsSlice = append(permittedFormatsSlice, format)
	}
	sort.StringSlice(permittedFormatsSlice).Sort()

	for sortBy := range permittedSortBy {
		permittedSortBySlice = append(permittedSortBySlice, sortBy)
	}
	sort.StringSlice(permittedSortBySlice).Sort()
}

// PermittedFormats list which formats are supported
func PermittedFormats() []string {
	return permittedFormatsSlice
}

// PermittedSortBy sorts all permitted sorting orders
var permittedSortBy = map[string]results.SortOrder{
	"bytes":   results.SortTraffic,
	"packets": results.SortPackets,
	"time":    results.SortTime,
}

// PermittedSortBy lists which sort by methods are supported
func PermittedSortBy() []string {
	return permittedSortBySlice
}
