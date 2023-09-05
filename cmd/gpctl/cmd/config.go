/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/cmd/gpctl/pkg/conf"
	"github.com/els0r/goProbe/pkg/api/goprobe/client"
	"github.com/els0r/goProbe/pkg/types/shellformat"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xlab/tablewriter"
)

const (
	flagFile   = "file"
	flagReload = "reload"
	flagSilent = "silent"

	defaultRequestTimeout = 3 * time.Second
)

var (
	file   string
	silent bool
	reload bool
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config [IFACES]",
	Short: "Get or update goProbe's runtime configuration",
	Long: `Get or update goProbe's runtime configuration

If the (list of) interface(s) is provided as an argument, it will only
show the configuration for them. Otherwise, all configurations are printed

The list of interfaces is ignored if -f|--file or -r|--reload is provided (which
are mutually exclusive and both trigger a change of goprobe's runtime configuration,
either from the provided file or reloading the on-disk configuration).
`,
	RunE:          wrapCancellationContext(configEntrypoint),
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.Flags().StringVarP(&file, flagFile, "f", "", "apply config file to goprobe's runtime configuration")
	configCmd.Flags().BoolVarP(&reload, flagReload, "r", false, "reload on-disk config file and apply it to goprobe's runtime configuration")
	configCmd.Flags().BoolVar(&silent, flagSilent, false, "don't output interface changes after update")
}

func configEntrypoint(ctx context.Context, cmd *cobra.Command, args []string) error {
	client := client.New(viper.GetString(conf.GoProbeServerAddr))

	// If the user specifies both a reload from disk and provided a config, abort (and show usage)
	if file != "" && reload {
		cmd.SilenceUsage = false
		return errors.New("cannot perform both config reload from disk and apply external runtime configuration")
	}
	if reload {
		return reloadConfig(ctx)
	}
	if file != "" {
		return updateConfig(ctx, file, silent)
	}

	ifaces := args

	ifaceConfigs, err := client.GetInterfaceConfig(ctx, ifaces...)
	if err != nil {
		return fmt.Errorf("failed to fetch config for interfaces %v: %w", ifaces, err)
	}

	var allConfigs []struct {
		iface string
		cfg   config.CaptureConfig
	}
	for iface, cfg := range ifaceConfigs {
		allConfigs = append(allConfigs, struct {
			iface string
			cfg   config.CaptureConfig
		}{
			iface: iface,
			cfg:   cfg,
		})
	}

	sort.SliceStable(allConfigs, func(i, j int) bool {
		return allConfigs[i].iface < allConfigs[j].iface
	})

	fmt.Println()

	table := tablewriter.CreateTable()
	table.UTF8Box()
	table.AddTitle(shellformat.FormatShell("Interface Configuration", shellformat.Bold))

	table.AddRow("", "", "ring buffer", "ring buffer")
	table.AddRow("iface", "promisc", "block size", "num blocks")
	table.AddSeparator()

	for _, icfg := range allConfigs {
		table.AddRow(icfg.iface,
			icfg.cfg.Promisc,
			icfg.cfg.RingBuffer.BlockSize,
			icfg.cfg.RingBuffer.NumBlocks,
		)
	}

	// set alignment before rendering
	table.SetAlign(tablewriter.AlignLeft, 1)
	table.SetAlign(tablewriter.AlignLeft, 2)
	table.SetAlign(tablewriter.AlignRight, 3)
	table.SetAlign(tablewriter.AlignRight, 4)

	fmt.Println(table.Render())

	return nil
}

func reloadConfig(ctx context.Context) error {
	client := client.New(viper.GetString(conf.GoProbeServerAddr))

	// send update call
	enabled, updated, disabled, err := client.ReloadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to trigger reload of goprobe's runtime configuration: %w", err)
	}

	if !silent {
		printIfaceChanges(enabled, updated, disabled)
	}

	return nil
}

func updateConfig(ctx context.Context, file string, silent bool) error {
	client := client.New(viper.GetString(conf.GoProbeServerAddr))

	// get the config from disk
	gpConfig, err := config.ParseFile(file)
	if err != nil {
		return fmt.Errorf("failed to load goprobe's config file %s: %w", file, err)
	}

	// validate config before sending it
	err = gpConfig.Validate()
	if err != nil {
		return fmt.Errorf("invalid configuration provided: %w", err)
	}

	// send update call
	enabled, updated, disabled, err := client.UpdateInterfaceConfigs(ctx, gpConfig.Interfaces)
	if err != nil {
		return fmt.Errorf("failed to update goprobe's runtime configuration: %w", err)
	}

	if !silent {
		printIfaceChanges(enabled, updated, disabled)
	}

	return nil
}

func printIfaceChanges(enabled, updated, disabled []string) {
	fmt.Printf(`
     Enabled: %v
     Updated: %v
    Disabled: %v

`, enabled, updated, disabled,
	)
}
