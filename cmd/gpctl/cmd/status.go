/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/els0r/goProbe/cmd/gpctl/pkg/conf"
	"github.com/els0r/goProbe/pkg/api/goprobe/client"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/formatting"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xlab/tablewriter"
)

// statusCmd represents the stats command
var statusCmd = &cobra.Command{
	Use:   "status [IFACES]",
	Short: "Show capture status",
	Long: `Show capture status

If the (list of) interface(s) is provided as an argument, it will only
show the statistics for them. Otherwise, all interfaces are printed
`,

	RunE: wrapCancellationContext(time.Second, statusEntrypoint),
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func statusEntrypoint(ctx context.Context, cmd *cobra.Command, args []string) error {
	client := client.New(viper.GetString(conf.GoProbeServerAddr))

	ifaces := args

	statuses, lastWriteout, err := client.GetInterfaceStatus(ctx, ifaces...)
	if err != nil {
		return fmt.Errorf("failed to fetch status for interfaces %v: %w", ifaces, err)
	}

	var (
		runtimeTotalReceived, runtimeTotalProcessed int64
		totalReceived, totalProcessed, totalDropped int64
		totalActive, totalIfaces                    int = 0, len(statuses)
	)

	fmt.Println()

	var allStatuses []struct {
		iface  string
		status capturetypes.CaptureStats
	}
	for iface, status := range statuses {
		allStatuses = append(allStatuses, struct {
			iface  string
			status capturetypes.CaptureStats
		}{
			iface:  iface,
			status: status,
		})
	}

	sort.SliceStable(allStatuses, func(i, j int) bool {
		return allStatuses[i].iface < allStatuses[j].iface
	})

	bold := color.New(color.Bold, color.FgWhite)
	boldRed := color.New(color.Bold, color.FgRed)

	table := tablewriter.CreateTable()
	table.UTF8Box()
	table.AddTitle(bold.Sprint("Interface Statuses"))

	table.AddRow("", "total", "", "total", "", "", "active")
	table.AddRow("iface",
		"received", "+ received",
		"processed", "+ processed",
		"dropped", "since",
	)
	table.AddSeparator()

	for _, st := range allStatuses {
		ifaceStatus := st.status

		runtimeTotalReceived += int64(ifaceStatus.ReceivedTotal)
		runtimeTotalProcessed += int64(ifaceStatus.ProcessedTotal)

		totalProcessed += int64(ifaceStatus.Processed)
		totalReceived += int64(ifaceStatus.Received)
		totalDropped += int64(ifaceStatus.Dropped)
		totalActive++

		dropped := fmt.Sprint(ifaceStatus.Dropped)
		if ifaceStatus.Dropped > 0 {
			dropped = boldRed.Sprint(ifaceStatus.Dropped)
		}

		table.AddRow(st.iface,
			formatting.Countable(ifaceStatus.ReceivedTotal), formatting.Countable(ifaceStatus.Received),
			formatting.Countable(ifaceStatus.ProcessedTotal), formatting.Countable(ifaceStatus.Processed),
			dropped,
			time.Since(ifaceStatus.StartedAt).Round(time.Second).String(),
		)
	}

	// set alignment before rendering
	table.SetAlign(tablewriter.AlignLeft, 1)
	table.SetAlign(tablewriter.AlignRight, 2)
	table.SetAlign(tablewriter.AlignRight, 3)
	table.SetAlign(tablewriter.AlignRight, 4)
	table.SetAlign(tablewriter.AlignRight, 5)
	table.SetAlign(tablewriter.AlignRight, 6)

	fmt.Println(table.Render())

	lastWriteoutStr := "-"
	ago := "-"
	if !lastWriteout.IsZero() {
		tLocal := lastWriteout.Local()

		lastWriteoutStr = tLocal.Format(types.DefaultTimeOutputFormat)
		ago = time.Since(tLocal).Round(time.Second).String()
	}

	fmt.Printf("%d/%d interfaces active\n\n", totalActive, totalIfaces)
	fmt.Printf(`Totals:
    Last writeout: %s (%s ago)
    Packets
       Received: %s / + %s
      Processed: %s / + %s
        Dropped: + %d

`, lastWriteoutStr, ago,
		formatting.Countable(runtimeTotalReceived), formatting.Countable(totalReceived),
		formatting.Countable(runtimeTotalProcessed), formatting.Countable(totalProcessed),
		totalDropped,
	)

	return nil
}
