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
	// summary, err := goDB.ReadDBSummary(dbPath)
	// if err != nil {
	// 	return err
	// }

	// if external {
	// 	if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
	// 		return err
	// 	}
	// } else {

	// 	wtxt := tabwriter.NewWriter(os.Stdout, 0, 4, 4, ' ', tabwriter.AlignRight)
	// 	fmt.Fprintln(wtxt, "")
	// 	fmt.Fprintln(wtxt, "Iface\t# of flows\tTraffic\tFrom\tUntil\t")
	// 	fmt.Fprintln(wtxt, "---------\t----------\t---------\t-------------------\t-------------------\t")

	// 	tunnelInfos := util.TunnelInfos()

	// 	ifaces := make([]string, 0, len(summary.Interfaces))
	// 	for iface := range summary.Interfaces {
	// 		ifaces = append(ifaces, iface)
	// 	}
	// 	sort.Strings(ifaces)

	// 	totalFlowCount, totalTraffic := uint64(0), uint64(0)
	// 	for _, iface := range ifaces {
	// 		ifaceDesc := iface
	// 		if ti, haveTunnelInfo := tunnelInfos[iface]; haveTunnelInfo {
	// 			ifaceDesc = fmt.Sprintf("%s (%s: %s)",
	// 				iface,
	// 				ti.PhysicalIface,
	// 				ti.Peer,
	// 			)
	// 		}

	// 		is := summary.Interfaces[iface]

	// 		tf := results.NewTextFormatter()
	// 		fmt.Fprintf(wtxt, "%s\t%s\t%s\t%s\t%s\t\n",
	// 			ifaceDesc,
	// 			tf.Count(is.Traffic.NumFlows()),
	// 			tf.Size(is.Counts.SumBytes()),
	// 			time.Unix(is.Begin, 0).Format("2006-01-02 15:04:05"),
	// 			time.Unix(is.End, 0).Format("2006-01-02 15:04:05"))

	// 		totalFlowCount += is.Traffic.NumFlows()
	// 		totalTraffic += is.Counts.SumBytes()
	// 	}
	// 	tf := results.NewTextFormatter()
	// 	fmt.Fprintln(wtxt, "\t \t \t \t \t")
	// 	fmt.Fprintf(wtxt, "Total\t%s\t%s\t\t\t\n",
	// 		tf.Count(totalFlowCount),
	// 		tf.Size(totalTraffic))
	// 	wtxt.Flush()
	// }
	// if !external {
	// 	fmt.Println()
	// }
	return nil
}
