package cmd

import (
	"fmt"
	"os"

	"github.com/els0r/goProbe/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/els0r/telemetry/logging"
	"github.com/els0r/telemetry/tracing"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var (
	cmdLineParams = &query.Args{}
	argsLocation  string
)

var shortText = "Query server for running distributed goQuery queries and aggregating the results"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "global-query [flags] [" + supportedCmds + "]",
	Short:         helpBase,
	Long:          helpBase,
	RunE:          func(cmd *cobra.Command, args []string) error { return nil },
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute is the main entrypoint and runs the CLI tool
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		logger, logErr := logging.New(logging.LevelError, logging.EncodingPlain,
			logging.WithOutput(os.Stderr),
		)
		if logErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to instantiate CLI logger: %v\n", logErr)

			fmt.Fprintf(os.Stderr, "Error running query: %s\n", err)
			os.Exit(1)
		}
		logger.Fatalf("Error running query: %s", err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(initLogger)

	// help commands
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultHelpFlag()

	pflags := rootCmd.PersistentFlags()

	tracing.RegisterFlags(pflags)

	pflags.String(conf.LogLevel, conf.DefaultLogLevel, "log level for logger")
	pflags.String(conf.LogEncoding, conf.DefaultLogEncoding, "message encoding format for logger")
	pflags.String(conf.HostsResolverType, conf.DefaultHostsResolver, "resolver used for the hosts resolution query")
	pflags.String(conf.QuerierType, conf.DefaultHostsQuerierType, "querier used to run queries")
	pflags.String(conf.QuerierConfig, "", "querier config file location")
	pflags.Int(conf.QuerierMaxConcurrent, 0, "maximum number of concurrent queries to hosts")

	pflags.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.global-query.yaml)")

	_ = viper.BindPFlags(rootCmd.Flags())
	_ = viper.BindPFlags(pflags)
}

func initLogger() {
	// since this is a command line tool, only warnings and errors should be printed and they
	// shouldn't go to a dedicated file
	err := logging.Init(logging.LevelFromString(viper.GetString(conf.LogLevel)), logging.Encoding(viper.GetString(conf.LogEncoding)),
		logging.WithVersion(version.Short()),
		logging.WithOutput(os.Stdout),
		logging.WithErrorOutput(os.Stderr),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cmd" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".cmd")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read in config: %v\n", err)
		os.Exit(1)
	}
}

func initHostListResolver() (hosts.Resolver, error) {
	resolverType := viper.GetString(conf.HostsResolverType)
	switch resolverType {
	case string(hosts.StringResolverType):
		return hosts.NewStringResolver(true), nil
	default:
		err := fmt.Errorf("hosts resolver type %q not supported", resolverType)
		return nil, err
	}
}
