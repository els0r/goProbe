// package conf enumerates the configuration options for the global query service
package conf

import (
	"errors"
	"fmt"
	"time"

	"github.com/els0r/telemetry/tracing"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// ServiceName is the name of the service as it will show up in telemetry such as metrics, logs, traces, etc.
	ServiceName = "global_query"
)

// Definitions for command line parameters / arguments
const (
	loggingKey  = "logging"
	LogLevel    = loggingKey + ".level"
	LogEncoding = loggingKey + ".encoding"

	profilingKey     = "profiling"
	ProfilingEnabled = profilingKey + ".enabled"

	hostsKey            = "hosts"
	hostsResolverKey    = hostsKey + ".resolver"
	HostsResolverType   = hostsResolverKey + ".type"
	HostsResolverConfig = hostsResolverKey + ".config"

	querierKey = "querier"

	QuerierType          = querierKey + ".type"
	QuerierConfig        = querierKey + ".config"
	QuerierMaxConcurrent = querierKey + ".max_concurrent"

	serverKey                 = "server"
	ServerAddr                = serverKey + ".addr"
	ServerShutdownGracePeriod = serverKey + ".shutdowngraceperiod"

	openapiKey         = "openapi"
	OpenAPISpecOutfile = openapiKey + ".spec-outfile"
)

// Global defaults for command line parameters / arguments
const (
	DefaultLogLevel    = "info"
	DefaultLogEncoding = "logfmt"

	DefaultHostsResolverType = "string"

	DefaultHostsQuerierType = "api"

	DefaultServerAddr                = "localhost:8145"
	DefaultServerShutdownGracePeriod = 30 * time.Second
)

// RegisterFlags registers all command line flags for the configuration
func RegisterFlags(cmd *cobra.Command, cfgFile *string) error {
	pflags := cmd.PersistentFlags()

	tracing.RegisterFlags(pflags)

	pflags.String(LogLevel, DefaultLogLevel, "log level for logger")
	pflags.String(LogEncoding, DefaultLogEncoding, "message encoding format for logger")
	pflags.String(HostsResolverType, DefaultHostsResolverType, "resolver used for the hosts resolution query")
	pflags.String(QuerierType, DefaultHostsQuerierType, "querier used to run queries")
	pflags.String(QuerierConfig, "", "querier config file location")
	pflags.Int(QuerierMaxConcurrent, 0, "maximum number of concurrent queries to hosts")

	if cfgFile == nil {
		return errors.New("config file pointer must not be nil")
	}
	pflags.StringVarP(cfgFile, "config", "c", "", "config file (default is $HOME/.global-query.yaml)")

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return fmt.Errorf("failed to bind flags: %w", err)
	}
	if err := viper.BindPFlags(pflags); err != nil {
		return fmt.Errorf("failed to bind persistent flags: %w", err)
	}
	return nil
}
