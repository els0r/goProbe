package cmd

import (
	"fmt"

	"github.com/els0r/goProbe/v4/pkg/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(*cobra.Command, []string) {
			printVersion()
		},
		SilenceErrors: true,
	}
}

func printVersion() {
	fmt.Printf("%s", version.Version())
}
