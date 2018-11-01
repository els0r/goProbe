/////////////////////////////////////////////////////////////////////////////////
//
// list.go
//
// Written by Fabian Kohn   fko@open.ch and
//            Lennart Elsen lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, September 2015
// Copyright (c) 2014 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import (
	"OSAG/goDB"
	"OSAG/util"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"
)

// List interfaces for which data is available and show how many flows and
// how much traffic was observed for each one.
func listInterfaces(dbPath string, external bool) error {
	summary, err := goDB.ReadDBSummary(dbPath)
	if err != nil {
		return err
	}

	if external {
		if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
			return err
		}
	} else {

		wtxt := tabwriter.NewWriter(os.Stdout, 0, 4, 4, ' ', tabwriter.AlignRight)
		fmt.Fprintln(wtxt, "")
		fmt.Fprintln(wtxt, "Iface\t# of flows\tTraffic\tFrom\tUntil\t")
		fmt.Fprintln(wtxt, "---------\t----------\t---------\t-------------------\t-------------------\t")

		tunnelInfos := util.TunnelInfos()

		ifaces := make([]string, 0, len(summary.Interfaces))
		for iface := range summary.Interfaces {
			ifaces = append(ifaces, iface)
		}
		sort.Strings(ifaces)

		totalFlowCount, totalTraffic := uint64(0), uint64(0)
		for _, iface := range ifaces {
			ifaceDesc := iface
			if ti, haveTunnelInfo := tunnelInfos[iface]; haveTunnelInfo {
				ifaceDesc = fmt.Sprintf("%s (%s: %s)",
					iface,
					ti.PhysicalIface,
					ti.Peer,
				)
			}

			is := summary.Interfaces[iface]

			fmt.Fprintf(wtxt, "%s\t%s\t%s\t%s\t%s\t\n",
				ifaceDesc,
				TextFormatter{}.Count(is.FlowCount),
				TextFormatter{}.Size(is.Traffic),
				time.Unix(is.Begin, 0).Format("2006-01-02 15:04:05"),
				time.Unix(is.End, 0).Format("2006-01-02 15:04:05"))
			totalFlowCount += is.FlowCount
			totalTraffic += is.Traffic
		}
		fmt.Fprintln(wtxt, "\t \t \t \t \t")
		fmt.Fprintf(wtxt, "Total\t%s\t%s\t\t\t\n",
			TextFormatter{}.Count(totalFlowCount),
			TextFormatter{}.Size(totalTraffic))
		wtxt.Flush()
	}
	return nil
}
