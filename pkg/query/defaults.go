package query

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
	DefaultSortBy         = "bytes"
)

// MaxResults stores the maximum number of rows a query will return. This limit is more or less
// theoretical, since a DB will unlikley feature such an amount of entries
const MaxResults = 9999999999999999

// PermittedFormats stores all supported output formats
var PermittedFormats = map[string]struct{}{
	"txt":      struct{}{},
	"json":     struct{}{},
	"csv":      struct{}{},
	"influxdb": struct{}{},
}

// PermittedSortBy sorts all permitted sorting orders
var PermittedSortBy = map[string]SortOrder{
	"bytes":   SortTraffic,
	"packets": SortPackets,
	"time":    SortTime,
}
