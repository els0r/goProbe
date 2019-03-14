package commands

import (
	"fmt"

	"github.com/els0r/goProbe/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print goProbe's and goQuery's version",
	Run: func(cmd *cobra.Command, args []string) {
		printVersion()
	},
}

func printVersion() {
	fmt.Printf("goProbe/goQuery %s\n", version.VersionText())
}
