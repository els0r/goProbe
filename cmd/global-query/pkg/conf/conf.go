package conf

import "time"

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
