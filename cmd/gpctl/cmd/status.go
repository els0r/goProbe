/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/els0r/goProbe/cmd/gpctl/pkg/conf"
	"github.com/els0r/goProbe/pkg/api/goprobe/client"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/status"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

const (
	receivedCol = "RECEIVED"
	droppedCol  = "DROPPED"

	colDistance = 4
)

func printHeader() {
	fmt.Println(strings.Repeat(" ", 2+status.StatusLineIndent+8+1) + receivedCol + strings.Repeat(" ", colDistance) + droppedCol)
}

func printStats(stats capturetypes.PacketStats) {
	rcvdStr := fmt.Sprint(stats.Received)
	droppedStr := fmt.Sprint(stats.Dropped)

	status.Okf("%s%s%s", rcvdStr, strings.Repeat(" ", len(receivedCol)+colDistance-len(rcvdStr)), droppedStr)
}

func statusEntrypoint(ctx context.Context, cmd *cobra.Command, args []string) error {
	client := client.New(viper.GetString(conf.GoProbeServerAddr))

	ifaces := args

	statuses, lastWriteout, err := client.GetInterfaceStatus(ctx, ifaces...)
	if err != nil {
		return fmt.Errorf("failed to fetch status for interfaces %v: %w", ifaces, err)
	}

	var (
		totalReceived, totalDropped int64
		totalActive, totalIfaces    int = 0, len(statuses)
	)

	fmt.Println()
	printHeader()

	var allStatuses []struct {
		iface  string
		status capturetypes.InterfaceStatus
	}
	for iface, status := range statuses {
		allStatuses = append(allStatuses, struct {
			iface  string
			status capturetypes.InterfaceStatus
		}{
			iface:  iface,
			status: status,
		})
	}

	sort.SliceStable(allStatuses, func(i, j int) bool {
		return allStatuses[i].iface < allStatuses[j].iface
	})

	for _, st := range allStatuses {
		status.Line(st.iface)

		ifaceStatus := st.status

		totalReceived += int64(ifaceStatus.PacketStats.Received)
		totalDropped += int64(ifaceStatus.PacketStats.Dropped)

		switch st.status.State {
		case capturetypes.StateError:
			status.Fail(ifaceStatus.State.String())
			continue
		case capturetypes.StateCapturing:
			totalActive++
		}
		printStats(ifaceStatus.PacketStats)
	}

	lastWriteoutStr := "-"
	ago := "-"
	if !lastWriteout.IsZero() {
		tLocal := lastWriteout.Local()

		lastWriteoutStr = tLocal.Format(types.DefaultTimeOutputFormat)
		ago = time.Since(tLocal).Round(time.Second).String()
	}

	fmt.Println()
	fmt.Printf("%d/%d interfaces active\n\n", totalActive, totalIfaces)
	fmt.Printf(`Totals:
    Last writeout: %s (%s ago)
    Packets
      Received: %d
      Dropped:  %d		(%.2f %%)

`, lastWriteoutStr, ago,
		totalReceived, totalDropped, float64(totalDropped)/float64(totalReceived)*100)

	return nil
}