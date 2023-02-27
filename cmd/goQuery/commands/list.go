package commands

import (
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists available interfaces and their statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listInterfaces(subcmdLineParams.DBPath, subcmdLineParams.External)
	},
}

// List interfaces for which data is available and show how many flows and
// how much traffic was observed for each one.
func listInterfaces(dbPath string, external bool) error {

	// TODO: Re-implement using information from new metadata
	return nil
}
