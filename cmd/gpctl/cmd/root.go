package cmd

import (
	"fmt"
	"net"
	"os"

	"github.com/els0r/goProbe/cmd/gpctl/pkg/conf"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gpctl",
	Short: "goProbe control CLI tool",
	Long: `gpctl goProbe control CLI tool

	TODO
`,
	PersistentPreRunE: verifyArgs,
	RunE:              rootEntrypoint,
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

			fmt.Fprintf(os.Stderr, "Error running command: %s\n", err)
			os.Exit(1)
		}
		logger.Fatalf("Error running command: %s", err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(initLogger)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gpctl.yaml)")

	rootCmd.PersistentFlags().StringP(conf.GoProbeServerAddr, "s", "", "server address of goProbe API")

	_ = viper.BindPFlags(rootCmd.PersistentFlags())
}

func initLogger() {
	// since this is a command line tool, only warnings and errors should be printed and they
	// shouldn't go to a dedicated file
	err := logging.Init(logging.LevelWarn, logging.EncodingLogfmt,
		logging.WithVersion(version.Short()),
		logging.WithOutput(os.Stdout),
		logging.WithErrorOutput(os.Stderr),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
}

// initConfig reads in config file and ENV variables if set. goQuery doesn't need one to run
// as a CLI tool. The functionality exists to set some defaults for e.g. the query-server
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)

		viper.AutomaticEnv() // read in environment variables that match

		// If a config file is found, read it in.
		if err := viper.ReadInConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read in config: %v\n", err)
			os.Exit(1)
		}
	}
}

func verifyArgs(_ *cobra.Command, _ []string) error {
	serverAddr := viper.GetString(conf.GoProbeServerAddr)
	if serverAddr == "" {
		return fmt.Errorf("%s: empty", conf.GoProbeServerAddr)
	}

	_, _, err := net.SplitHostPort(serverAddr)
	if err != nil {
		return fmt.Errorf("%s: %w", conf.GoProbeServerAddr, err)
	}
	return nil
}

func rootEntrypoint(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("no sub-command provided")
}
