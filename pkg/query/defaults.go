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

func PermittedFormats() []string {
	var aux []string
	for format := range permittedFormats {
		aux = append(aux, format)
	}
	sort.StringSlice(aux).Sort()
	return aux
}

// PermittedSortBy sorts all permitted sorting orders
var permittedSortBy = map[string]results.SortOrder{
	"bytes":   results.SortTraffic,
	"packets": results.SortPackets,
	"time":    results.SortTime,
}

func PermittedSortBy() []string {
	var aux []string
	for format := range permittedSortBy {
		aux = append(aux, format)
	}
	sort.StringSlice(aux).Sort()
	return aux
}
