package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/els0r/goProbe/cmd/gpctl/pkg/conf"
	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/els0r/telemetry/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const defaultCfgFile = "~/.gpctl.yaml"

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:               "gpctl",
	Short:             "goProbe control CLI tool",
	Long:              `gpctl goProbe control CLI tool`,
	PersistentPreRunE: verifyArgs,
	RunE:              rootEntrypoint,
	SilenceErrors:     true,
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", defaultCfgFile, "config file")

	rootCmd.PersistentFlags().StringP(conf.GoProbeServerAddr, "s", "", "server address of goProbe API")
	rootCmd.PersistentFlags().DurationP(conf.RequestTimeout, "t", defaultRequestTimeout, "request timeout / deadline for goProbe API")

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
			// If no non-default config file was set and it doesn't exist, silently exit and proceed
			if cfgFile == defaultCfgFile && errors.Is(err, os.ErrNotExist) {
				return
			}
			fmt.Fprintf(os.Stderr, "failed to read config from file %s: %v\n", viper.GetViper().ConfigFileUsed(), err)
			os.Exit(1)
		}
	}
}

func verifyArgs(cmd *cobra.Command, _ []string) error {
	// don't verify server if the help has been requested or the version
	// should be printed
	if cmd.Use == "help" || cmd.Use == versionCmd.Use {
		return nil
	}

	serverAddr := viper.GetString(conf.GoProbeServerAddr)
	if serverAddr == "" {
		return fmt.Errorf("%s: empty", conf.GoProbeServerAddr)
	}

	unixSocketFile := api.ExtractUnixSocket(serverAddr)
	if unixSocketFile != "" {
		return nil
	}

	_, _, err := net.SplitHostPort(serverAddr)
	if err != nil {
		return fmt.Errorf("%s: %w", conf.GoProbeServerAddr, err)
	}
	return nil
}

func rootEntrypoint(_ *cobra.Command, _ []string) error {
	return fmt.Errorf("no sub-command provided")
}

type entrypointE func(ctx context.Context, cmd *cobra.Command, args []string) error
type runE func(cmd *cobra.Command, args []string) error

func wrapCancellationContext(f entrypointE) runE {

	return func(cmd *cobra.Command, args []string) error {
		sdCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
		defer stop()

		// calls to the api shouldn't take longer than one second
		ctx, cancel := context.WithTimeout(sdCtx, viper.GetDuration(conf.RequestTimeout))
		defer cancel()

		return f(ctx, cmd, args)
	}
}
