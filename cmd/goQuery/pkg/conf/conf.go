package conf

// Definitions for command line parameters / arguments
const (
	queryKey = "query"

	serverKey            = queryKey + ".server"
	QueryServerAddr      = serverKey + ".addr"
	QueryTimeout         = queryKey + ".timeout"
	QueryHostsResolution = queryKey + ".hosts-resolution"
	QueryLog             = queryKey + ".log"
	QueryKeepAlive       = queryKey + ".keepalive"
	QueryStats           = queryKey + ".stats"
	QueryStreaming       = queryKey + ".streaming"

	dbKey       = "db"
	QueryDBPath = dbKey + ".path"

	StoredQuery = "stored-query"

	// logging
	loggingKey = "logging"
	LogLevel   = loggingKey + ".level"

	// DNS settings
	dnsKey               = "dns-resolution"
	DNSResolutionEnabled = dnsKey + ".enabled"
	DNSResolutionMaxRows = dnsKey + ".max-rows"
	DNSResolutionTimeout = dnsKey + ".timeout"

	// Sorting
	sortKey       = "sort"
	SortBy        = sortKey + ".by"
	SortAscending = sortKey + ".ascending"

	// Results
	resultsKey    = "results"
	ResultsFormat = resultsKey + ".format"
	ResultsLimit  = resultsKey + ".limit"

	// Memory
	memoryKey     = "memory"
	MemoryMaxPct  = memoryKey + ".max-pct"
	MemoryLowMode = memoryKey + ".low-mode"

	// Time
	First = "first"
	Last  = "last"

	// Profiling
	profilingKey       = "profiling"
	ProfilingOutputDir = profilingKey + ".output-dir"

	// Tracing propagation
	Traceparent = "traceparent"
)
