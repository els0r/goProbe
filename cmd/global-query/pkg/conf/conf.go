package conf

import "time"

const (
	// ServiceName is the name of the service as it will show up in telemetry such as metrics, logs, traces, etc.
	ServiceName = "global_query"
)

const (
	loggingKey  = "logging"
	LogLevel    = loggingKey + ".level"
	LogEncoding = loggingKey + ".encoding"

	profilingKey     = "profiling"
	ProfilingEnabled = profilingKey + ".enabled"

	hostsKey         = "hosts"
	hostsResolverKey = hostsKey + ".resolver"

	HostsResolverType = hostsResolverKey + ".type"

	querierKey = "querier"

	QuerierType          = querierKey + ".type"
	QuerierConfig        = querierKey + ".config"
	QuerierMaxConcurrent = querierKey + ".max_concurrent"

	serverKey                 = "server"
	ServerAddr                = serverKey + ".addr"
	ServerShutdownGracePeriod = serverKey + ".shutdowngraceperiod"
)

const (
	DefaultLogLevel    = "info"
	DefaultLogEncoding = "logfmt"

	DefaultHostsResolver = "string"

	DefaultHostsQuerierType = "api"

	DefaultServerAddr                = "localhost:8145"
	DefaultServerShutdownGracePeriod = 30 * time.Second
)
