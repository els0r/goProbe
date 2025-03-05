package cmd

import (
	"fmt"

	"github.com/els0r/goProbe/v4/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		printVersion()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func printVersion() {
	fmt.Printf("%s", version.Version())
}
