package query

// Defaults for query arguments
var (
	DefaultDBPath         = "/opt/ntm/goProbe/db"
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

var PermittedFormats = map[string]struct{}{
	"txt":      struct{}{},
	"json":     struct{}{},
	"csv":      struct{}{},
	"influxdb": struct{}{},
}

var PermittedSortBy = map[string]SortOrder{
	"bytes":   SortTraffic,
	"packets": SortPackets,
	"time":    SortTime,
}
