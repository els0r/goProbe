// Package cmd provides the runnable commands for global query
package cmd

import (
	"fmt"
	"os"

	"github.com/els0r/goProbe/v4/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/v4/pkg/version"
	"github.com/els0r/telemetry/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Execute is the main entrypoint and runs the CLI tool
func Execute() error {
	var cfgFile string

	// rootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use:   "global-query [flags] [" + supportedCmds + "]",
		Short: helpBase,
		Long:  helpBase,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return nil
		},
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	// help commands
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultHelpFlag()

	if err := conf.RegisterFlags(rootCmd, &cfgFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register flags: %v\n", err)
		os.Exit(1)
	}

	serverCmd, err := serverCommand()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(serverCmd)

	cobra.OnInitialize(func() { initConfig(&cfgFile) })
	cobra.OnInitialize(initLogger)

	return rootCmd.Execute()
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
func initConfig(cfgFile *string) {
	if cfgFile != nil {
		// Use config file from the flag.
		viper.SetConfigFile(*cfgFile)
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
