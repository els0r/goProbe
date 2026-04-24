package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/els0r/goProbe/v4/cmd/gpctl/pkg/conf"
	"github.com/els0r/goProbe/v4/pkg/api"
	"github.com/els0r/goProbe/v4/pkg/version"
	"github.com/els0r/telemetry/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Execute is the main entrypoint and runs the CLI tool
func Execute() {
	rootCmd := newRootCmd()
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newMergeCmd())

	err := rootCmd.Execute()
	if err != nil {
		logger, _, logErr := logging.New(logging.LevelError, logging.EncodingPlain,
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

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               "gpdb",
		Short:             "goProbe DB maintenance tool",
		Long:              `gpdb goProbe DB maintenance CLI tool`,
		PersistentPreRunE: rootPersistentPreRun,
		RunE:              rootEntrypoint,
		SilenceErrors:     true,
	}

	_ = viper.BindPFlags(rootCmd.PersistentFlags())

	return rootCmd
}

func rootPersistentPreRun(cmd *cobra.Command, args []string) error {
	initConfig()
	initLogger()
	return verifyArgs(cmd, args)
}

// initConfig is currently a no-op to keep parity with cobra initialization hooks.
func initConfig() {}

func initLogger() {
	// since this is a command line tool, only warnings and errors should be printed and they
	// shouldn't go to a dedicated file
	_, err := logging.Init(logging.LevelWarn, logging.EncodingLogfmt,
		logging.WithVersion(version.Short()),
		logging.WithOutput(os.Stdout),
		logging.WithErrorOutput(os.Stderr),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
}

func verifyArgs(cmd *cobra.Command, _ []string) error {
	// Don't verify server if command does not rely on API access.
	if cmd.Name() == "help" || cmd.Name() == "version" || cmd.Name() == "gpdb" || cmd.Name() == "merge" {
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

		timeout := viper.GetDuration(conf.RequestTimeout)
		var (
			ctx    context.Context
			cancel context.CancelFunc
		)
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(sdCtx, timeout)
		} else {
			ctx, cancel = context.WithCancel(sdCtx)
		}
		defer cancel()

		return f(ctx, cmd, args)
	}
}
