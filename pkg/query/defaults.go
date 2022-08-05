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

// PermittedFormats stores all supported output formats
var PermittedFormats = map[string]struct{}{
	"txt":      {},
	"json":     {},
	"csv":      {},
	"influxdb": {},
}

// PermittedSortBy sorts all permitted sorting orders
var PermittedSortBy = map[string]SortOrder{
	"bytes":   SortTraffic,
	"packets": SortPackets,
	"time":    SortTime,
}
