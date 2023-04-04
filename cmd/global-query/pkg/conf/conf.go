package conf

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
)

const (
	DefaultLogLevel    = "info"
	DefaultLogEncoding = "logfmt"

	DefaultHostsResolver = "string"

	DefaultHostsQuerierType = "api"
)
