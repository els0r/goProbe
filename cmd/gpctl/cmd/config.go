/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/cmd/gpctl/pkg/conf"
	"github.com/els0r/goProbe/pkg/api/goprobe/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	flagFile   = "file"
	flagSilent = "silent"
)

var (
	file   string
	silent bool
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config [IFACES]",
	Short: "Get or update goProbe's runtime configuration",
	Long: `Get or update goProbe's runtime configuration

If the (list of) interface(s) is provided as an argument, it will only
show the configuration for them. Otherwise, all configurations are printed

The list of interfaces is ignored if --file is provided to reload
goprobe's runtime configuration
`,
	RunE:          wrapCancellationContext(time.Second, configEntrypoint),
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.Flags().StringVarP(&file, flagFile, "f", "", "apply config file to goprobe's runtime configuration")
	configCmd.Flags().BoolVar(&silent, flagSilent, false, "don't output interface changes after update")
}

func configEntrypoint(ctx context.Context, cmd *cobra.Command, args []string) error {
	client := client.New(viper.GetString(conf.GoProbeServerAddr))

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
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', tabwriter.AlignRight)

	fmt.Fprintln(tw, "\tiface\tpromisc\tring_buffer_block_size\tring_buffer_num_blocks\t")
	for _, icfg := range allConfigs {
		fmt.Fprintf(tw, "\t%s\t%t\t%d\t%d\t\n", icfg.iface,
			icfg.cfg.Promisc,
			icfg.cfg.RingBufferBlockSize,
			icfg.cfg.RingBufferNumBlocks,
		)
	}
	tw.Flush()
	fmt.Println()

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

	if silent {
		return nil
	}

	fmt.Printf(`
     Enabled: %v
     Updated: %v
    Disabled: %v

`, enabled, updated, disabled,
	)

	return nil
}
