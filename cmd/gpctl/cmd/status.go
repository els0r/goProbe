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

const (
	flagExtended = "extended"
)

// statusCmd represents the stats command
var statusCmd = &cobra.Command{
	Use:   "status [IFACES]",
	Short: "Show capture status",
	Long: `Show capture status

If the (list of) interface(s) is provided as an argument, it will only
show the statistics for them. Otherwise, all interfaces are printed
`,

	RunE:          wrapCancellationContext(statusEntrypoint),
	SilenceErrors: true, // Errors are emitted after command completion, avoid duplicate
}

var extended bool

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().BoolVarP(&extended, flagExtended, "e", false, "print extended interface statistics (packet parsing errors)")
}

func statusEntrypoint(ctx context.Context, cmd *cobra.Command, args []string) error {
	client := client.New(viper.GetString(conf.GoProbeServerAddr))

	ifaces := args

	statuses, lastWriteout, startedAt, err := client.GetInterfaceStatus(ctx, ifaces...)
	if err != nil {

		// If the error is caused by context timeout / cancellation, skip the usage notification
		if errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, context.Canceled) {
			cmd.SilenceUsage = true
		}
		return fmt.Errorf("failed to fetch status for interfaces %v: %w", ifaces, err)
	}

	var (
		runtimeTotalReceived, runtimeTotalProcessed, runtimeTotalDropped int64
		totalReceived, totalProcessed, totalDropped                      int64
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

	headerRow1 := []interface{}{"", "total", "", "total", "", "total", "", "active"}
	headerRow2 := []interface{}{"iface",
		"received", "+ received",
		"processed", "+ processed",
		"dropped", "+ dropped", "for"}
	if extended {
		for _, parsingErrnoName := range capturetypes.ParsingErrnoNames {
			headerRow2 = append(headerRow2, parsingErrnoName)
		}
		headerRow1 = append(headerRow1, tablewriter.CreateCell("parsing errors", &tablewriter.CellStyle{
			Alignment: tablewriter.AlignCenter,
			ColSpan:   2,
		}))
	}

	table.AddRow(headerRow1...)
	table.AddRow(headerRow2...)
	table.AddSeparator()

	for _, st := range allStatuses {
		ifaceStatus := st.status

		runtimeTotalReceived += int64(ifaceStatus.ReceivedTotal)
		runtimeTotalProcessed += int64(ifaceStatus.ProcessedTotal)
		runtimeTotalDropped += int64(ifaceStatus.DroppedTotal)

		totalProcessed += int64(ifaceStatus.Processed)
		totalReceived += int64(ifaceStatus.Received)
		totalDropped += int64(ifaceStatus.Dropped)

		dropped := fmt.Sprint(formatting.Countable(ifaceStatus.Dropped))
		if ifaceStatus.Dropped > 0 {
			dropped = boldRed.Sprint(ifaceStatus.Dropped)
		}

		ifaceRow := []interface{}{st.iface,
			formatting.Countable(ifaceStatus.ReceivedTotal), formatting.Countable(ifaceStatus.Received),
			formatting.Countable(ifaceStatus.ProcessedTotal), formatting.Countable(ifaceStatus.Processed),
			formatting.Countable(ifaceStatus.DroppedTotal), dropped,
			time.Since(ifaceStatus.StartedAt).Round(time.Second).String()}
		if extended {
			for _, parsingErrno := range ifaceStatus.ParsingErrors {
				ifaceRow = append(ifaceRow, tablewriter.CreateCell(formatting.Countable(parsingErrno), &tablewriter.CellStyle{Alignment: tablewriter.AlignRight}))
			}
		}

		table.AddRow(ifaceRow...)
	}

	// set alignment before rendering
	table.SetAlign(tablewriter.AlignLeft, 1)
	for i := 2; i <= 8; i++ {
		table.SetAlign(tablewriter.AlignRight, i)
	}

	fmt.Println(table.Render())

	lastWriteoutStr := "-"
	ago := "-"
	if !lastWriteout.IsZero() {
		tLocal := lastWriteout.Local()

		lastWriteoutStr = tLocal.Format(types.DefaultTimeOutputFormat)
		ago = time.Since(tLocal).Round(time.Second).String()
	}

	fmt.Printf(`Runtime info:

            Running since: %s (%s ago)
  Last scheduled writeout: %s (%s ago)

Totals:

    Packets
       Received: %s / + %s
      Processed: %s / + %s
        Dropped: %s / + %s

`,
		startedAt.Local().Format(types.DefaultTimeOutputFormat), time.Since(startedAt).Round(time.Second).String(),
		lastWriteoutStr, ago,
		formatting.Countable(runtimeTotalReceived), formatting.Countable(totalReceived),
		formatting.Countable(runtimeTotalProcessed), formatting.Countable(totalProcessed),
		formatting.Countable(runtimeTotalDropped), formatting.Countable(totalDropped),
	)

	return nil
}
