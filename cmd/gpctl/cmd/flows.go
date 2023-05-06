package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/els0r/goProbe/cmd/gpctl/pkg/conf"
	"github.com/els0r/goProbe/pkg/api/goprobe/client"
	"github.com/els0r/goProbe/pkg/goprobe/types"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// flowsCmd represents the flows command
var flowsCmd = &cobra.Command{
	Use:   "flows",
	Short: "Print flows that are currently active",
	Long:  `Print flows that are currently active`,
	RunE:  wrapCancellationContext(time.Second, flowsEntrypoint),
}

func init() {
	rootCmd.AddCommand(flowsCmd)
}

func flowsEntrypoint(ctx context.Context, cmd *cobra.Command, args []string) error {
	client := client.New(viper.GetString(conf.GoProbeServerAddr))

	ifaces := args

	fir, err := client.GetActiveFlows(ctx, ifaces...)
	if err != nil {
		return fmt.Errorf("failed to query flows for interfaces %v: %w", ifaces, err)
	}

	var allFlowInfos []struct {
		iface string
		infos types.FlowInfos
	}
	for iface, infos := range fir.Flows {
		allFlowInfos = append(allFlowInfos, struct {
			iface string
			infos types.FlowInfos
		}{
			iface: iface,
			infos: infos,
		})
	}

	sort.SliceStable(allFlowInfos, func(i, j int) bool {
		return allFlowInfos[i].iface < allFlowInfos[j].iface
	})

	fmt.Println()

	for _, info := range allFlowInfos {
		fmt.Printf("%s (%d flows)\n\n", info.iface, len(info.infos))

		if len(info.infos) > 0 {
			err := info.infos.TablePrint(os.Stdout)
			if err != nil {
				logging.FromContext(ctx).Error("failed to print flow table: %v", err)
				fmt.Println()
				continue
			}
			fmt.Println()
		}
	}

	return nil
}
