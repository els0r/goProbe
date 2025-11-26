// Package conf provides shared configuration handling utilities for all services
package conf

import (
	"github.com/els0r/telemetry/tracing"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	ConfigFile = "config"

	loggingKey = "logging"

	LogDestination = loggingKey + ".destination"
	LogEncoding    = loggingKey + ".encoding"
	LogLevel       = loggingKey + ".level"
)

// Global defaults for command line parameters / arguments
const (
	DefaultLogEncoding = "logfmt"
	DefaultLogLevel    = "info"
)

// RegisterFlags registers all command line flags for the configuration
func RegisterFlags(cmd *cobra.Command) error {
	pflags := cmd.PersistentFlags()

	pflags.StringP(ConfigFile, "c", "", "path to configuration file")

	tracing.RegisterFlags(pflags)

	pflags.String(LogLevel, DefaultLogLevel, "log level for logger")
	pflags.String(LogEncoding, DefaultLogEncoding, "message encoding format for logger")
	pflags.String(LogDestination, "", "logging destination file path (empty for stdout)")

	return viper.BindPFlags(pflags)
}
