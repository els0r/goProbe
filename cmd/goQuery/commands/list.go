package commands

import (
	"github.com/els0r/goProbe/cmd/goQuery/pkg/conf"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists available interfaces and their statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listInterfaces(viper.GetString(conf.DBPath))
	},
}

// List interfaces for which data is available and show how many flows and
// how much traffic was observed for each one.
func listInterfaces(dbPath string) error {
	// TODO: Re-implement using information from new metadata
	return nil
}
