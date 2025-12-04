package cmd

import (
	"fmt"

	"github.com/els0r/goProbe/v4/pkg/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print goProbe's version and exit",
		Run: func(*cobra.Command, []string) {
			printVersion()
		},
	}
}
func printVersion() {
	fmt.Printf("%s\n", version.Version())
}
