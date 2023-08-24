package cmd

import (
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
	jsoniter "github.com/json-iterator/go"
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
	RunE: listInterfacesEntrypoint,
}

var detailed bool

func init() {
	rootCmd.AddCommand(listCmd)

	flags := listCmd.Flags()

	flags.BoolVarP(&detailed, "detailed", "v", false, `print more information in the list.

If enabled, both directions for packet and byte counters will be printed, the flows will
be broken up into IPv4 and IPv6 flows and the drops for that interface will be shown.
`)
}

func listInterfacesEntrypoint(cmd *cobra.Command, args []string) error {
	return listInterfaces(viper.GetString(conf.QueryDBPath), args...)
}

// List interfaces for which data is available and show how many flows and
// how much traffic was observed for each one.
func listInterfaces(dbPath string, ifaces ...string) error {
	queryArgs := cmdLineParams

	// TODO: consider making this configurable
	output := os.Stdout

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
		wm, err := goDB.NewDBWorkManager(goDB.NewMetadataQuery(), dbPath, iface, runtime.NumCPU())
		if err != nil {
			return fmt.Errorf("failed to set up work manager for %s: %w", iface, err)
		}
		dbWorkerManagers = append(dbWorkerManagers, wm)
	}

	var ifacesMetadata = make([]*goDB.InterfaceMetadata, 0, len(dbWorkerManagers))
	for _, manager := range dbWorkerManagers {
		manager := manager

		im, err := manager.ReadMetadata(first, last)
		if err != nil {
			return err
		}
		ifacesMetadata = append(ifacesMetadata, im)
	}

	if queryArgs.Format == "json" {
		return jsoniter.NewEncoder(output).Encode(ifacesMetadata)
	}

	// empty line before table header
	fmt.Println()

	// pretty print the results
	err = printInterfaceStats(output, ifacesMetadata, detailed)
	if err != nil {
		return err
	}

	// empty line at bottom
	fmt.Println()

	return nil
}

const (
	tableSep = ' '
	itemSep  = "\t"
)

func printInterfaceStats(w io.Writer, ifaceMetadata []*goDB.InterfaceMetadata, detailed bool) error {
	tw := tabwriter.NewWriter(w, 0, 4, 4, tableSep, tabwriter.AlignRight)

	if len(ifaceMetadata) == 0 {
		return errors.New("no metadata available for printing")
	}

	headers := ifaceMetadata[0].TableHeader(detailed)

	for _, headerRow := range headers {
		fmt.Fprintln(tw, strings.Join(headerRow, itemSep)+itemSep)
	}

	lowerHeaderRowInd := len(headers) - 1

	// compute the number of dashes to print for the table header
	fieldLengths := make([]int, len(headers[lowerHeaderRowInd]))
	for _, header := range headers {
		for i, field := range header {
			if len(field) > fieldLengths[i] {
				fieldLengths[i] = len(field)
			}
		}
	}
	seps := make([]string, 0, len(headers[lowerHeaderRowInd]))
	for _, fieldLength := range fieldLengths {
		seps = append(seps, strings.Repeat("-", fieldLength))
	}
	fmt.Fprintln(tw, strings.Join(seps, itemSep)+itemSep)

	var totalsMetadata = &goDB.InterfaceMetadata{}

	for _, metadata := range ifaceMetadata {
		fmt.Fprintln(tw, strings.Join(metadata.TableRow(detailed), itemSep)+itemSep)

		// sum across interfaces
		totalsMetadata.Stats = totalsMetadata.Add(metadata.Stats)
	}

	// empty row for the table. Just reuse the sep slice
	for i := range seps {
		seps[i] = ""
	}
	fmt.Fprintln(tw, strings.Join(seps, itemSep)+itemSep)

	// sum row
	sumRow := totalsMetadata.TableRow(detailed)
	// iface, from, to make no sense in the totals, so remove them
	sumRow[0] = "Total"
	if detailed {
		sumRow[8], sumRow[9] = "", ""
	} else {
		sumRow[4], sumRow[5] = "", ""
	}

	fmt.Fprintln(tw, strings.Join(sumRow, itemSep)+itemSep)

	return tw.Flush()
}
