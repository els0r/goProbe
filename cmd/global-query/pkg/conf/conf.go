package conf

import "time"

const (
	loggingKey  = "logging"
	LogLevel    = loggingKey + ".level"
	LogEncoding = loggingKey + ".encoding"

	hostsKey         = "hosts"
	hostsResolverKey = hostsKey + ".resolver"

	HostsResolverType = hostsResolverKey + ".type"

	hostsQuerierKey = hostsKey + ".querier"

	HostsQuerierType   = hostsQuerierKey + ".type"
	HostsQuerierConfig = hostsQuerierKey + ".config"

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
