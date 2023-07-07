package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/els0r/goProbe/cmd/goQuery/pkg/conf"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/info"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list [ifaces]",
	Short: "Lists available interfaces and their statistics",
	Long: `Lists available interfaces and their statistics

If a list of interfaces is provided, the statistics for those interfaces
are printed.

Otherwise, all interface statistics are printed. By default, cumulative
information is printed (sum of flows, sum of ingress and egress traffic)
`,
	RunE: func(_ *cobra.Command, args []string) error {
		return listInterfaces(viper.GetString(conf.QueryDBPath), args...)
	},
}

var detailed bool

func init() {
	flags := rootCmd.Flags()

	flags.BoolVar(&detailed, "detailed", false, `print more information in the list.

If enabled, both directions for packet and byte counters will be printed, the flows will
be broken up into IPv4 and IPv6 flows and the drops for that interface will be shown.`)
}

// List interfaces for which data is available and show how many flows and
// how much traffic was observed for each one.
func listInterfaces(dbPath string, ifaces ...string) error {
	queryArgs := cmdLineParams

	// TODO: consider making this configurable
	output := os.Stdout

	fmt.Println("first", queryArgs.First, "last", queryArgs.Last)
	first, last, err := query.ParseTimeRange(queryArgs.First, queryArgs.Last)
	if err != nil {
		return err
	}

	ifaceDirs, err := info.GetInterfaces(dbPath)
	if err != nil {
		return err
	}

	// filter by provided interfaces. If the name isn't found in the
	// directory, ignore it
	if len(ifaces) > 0 {
		var auxIfaces = make(map[string]struct{})
		for _, iface := range ifaces {
			auxIfaces[iface] = struct{}{}
		}

		var candidates []string
		for _, iface := range ifaceDirs {
			_, exists := auxIfaces[iface]
			if exists {
				candidates = append(candidates, iface)
			}
		}
		ifaceDirs = candidates
	}

	// create work managers
	var dbWorkerManagers = make([]*goDB.DBWorkManager, 0, len(ifaceDirs))
	for _, iface := range ifaceDirs {
		wm, err := goDB.NewDBWorkManager(dbPath, iface, runtime.NumCPU())
		if err != nil {
			return fmt.Errorf("failed to set up work manager for %s: %w", iface, err)
		}
		dbWorkerManagers = append(dbWorkerManagers, wm)
	}

	var ifacesMetadata = make([]*goDB.InterfaceMetadata, 0, len(dbWorkerManagers))
	for i, manager := range dbWorkerManagers {
		manager := manager

		ifacesMetadata[i], err = manager.ReadMetadata(first, last)
		if err != nil {
			return err
		}
	}

	if queryArgs.Format == "json" {
		err := json.NewEncoder(output).Encode(ifacesMetadata)
		if err != nil {
			return err
		}
	}

	// pretty print the results
	return printInterfaceStats(output, ifacesMetadata, detailed)
}

const (
	tableSep = '\t'
	itemSep  = string(tableSep)
)

func printInterfaceStats(w io.Writer, ifaceMetadata []*goDB.InterfaceMetadata, detailed bool) error {
	tw := tabwriter.NewWriter(w, 4, 4, 0, tableSep, tabwriter.AlignRight)

	if len(ifaceMetadata) == 0 {
		return errors.New("no metadata available for printing")
	}

	headers := ifaceMetadata[0].TableHeader(detailed)

	fmt.Fprintln(tw, strings.Join(headers[0], itemSep))
	fmt.Fprintln(tw, strings.Join(headers[1], itemSep))

	seps := make([]string, 0, len(headers[1]))
	for _, header := range headers[1] {
		seps = append(seps, strings.Repeat("-", len(header)))
	}
	fmt.Fprintln(tw, strings.Join(seps, itemSep))

	var totalsMetadata = &goDB.InterfaceMetadata{}

	for _, metadata := range ifaceMetadata {
		fmt.Fprintln(tw, strings.Join(metadata.TableRow(detailed), itemSep))

		// sum across interfaces
		totalsMetadata.Stats = totalsMetadata.Add(metadata.Stats)
	}

	// empty row for the table. Just reuse the sep slice
	for i := range seps {
		seps[i] = ""
	}
	fmt.Fprintln(tw, strings.Join(seps, itemSep))

	// sum row
	sumRow := totalsMetadata.TableRow(detailed)
	// iface, from, to make no sense in the totals, so remove them
	sumRow[0], sumRow[1], sumRow[2] = "", "", ""

	fmt.Fprintln(tw, strings.Join(sumRow, itemSep))

	return tw.Flush()
}
