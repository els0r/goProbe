package cmd

import (
	"fmt"
	"os"

	"github.com/els0r/goProbe/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/cmd/global-query/pkg/distributed"
	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/version"
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

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(initLogger)

	// help commands
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultHelpFlag()

	rootCmd.PersistentFlags().String(conf.LogLevel, conf.DefaultLogLevel, "log level for logger")
	rootCmd.PersistentFlags().String(conf.LogEncoding, conf.DefaultLogEncoding, "message encoding format for logger")
	rootCmd.PersistentFlags().String(conf.HostsResolverType, conf.DefaultHostsResolver, "resolver used for the hosts resolution query")
	rootCmd.PersistentFlags().String(conf.QuerierType, conf.DefaultHostsQuerierType, "querier used to run queries")
	rootCmd.PersistentFlags().String(conf.QuerierConfig, "", "querier config file location")
	rootCmd.PersistentFlags().Int(conf.QuerierMaxConcurrent, 0, "maximum number of concurrent queries to hosts")

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.global-query.yaml)")

	_ = viper.BindPFlags(rootCmd.Flags())
	_ = viper.BindPFlags(rootCmd.PersistentFlags())
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
	if err := viper.ReadInConfig(); err != nil {
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

func initQuerier() (distributed.Querier, error) {
	querierType := viper.GetString(conf.QuerierType)
	switch querierType {
	case string(distributed.APIClientQuerierType):
		return distributed.NewAPIClientQuerier(viper.GetString(conf.QuerierConfig))
	default:
		err := fmt.Errorf("querier type %q not supported", querierType)
		return nil, err
	}
}
