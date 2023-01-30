package query

import (
	"time"

	"github.com/els0r/goProbe/pkg/results"
)

// Defaults for query arguments
var (
	DefaultDBPath         = "/usr/local/goProbe/db"
	DefaultFormat         = "txt"
	DefaultIn             = false
	DefaultMaxMemPct      = 60
	DefaultNumResults     = 1000
	DefaultOut            = false
	DefaultResolveRows    = 25
	DefaultResolveTimeout = 1 // seconds
	DefaultQueryTimeout   = 0 * time.Second
	DefaultSortBy         = "bytes"
)

// PermittedFormats stores all supported output formats
var PermittedFormats = map[string]struct{}{
	"txt":  {},
	"json": {},
	"csv":  {},
}

// PermittedSortBy sorts all permitted sorting orders
var PermittedSortBy = map[string]results.SortOrder{
	"bytes":   results.SortTraffic,
	"packets": results.SortPackets,
	"time":    results.SortTime,
}
