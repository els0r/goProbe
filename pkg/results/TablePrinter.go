/////////////////////////////////////////////////////////////////////////////////
//
// TablePrinter.go
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package results

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/els0r/goProbe/pkg/formatting"
	"github.com/els0r/goProbe/pkg/goDB/protocols"
	"github.com/els0r/goProbe/pkg/types"
	"go.opentelemetry.io/otel/trace"
)

// OutputColumn ranges over all possible output columns.
// Not every format prints every output column, e.g. the InfluxDBTablePrinter
// completely ignorowall percentages.
type OutputColumn int

// Enumeration of all possible output columns
const (
	// labels
	OutcolTime OutputColumn = iota
	OutcolHostname
	OutcolHostID
	OutcolIface
	// attributes
	OutcolSIP
	OutcolDIP
	OutcolDport
	OutcolProto
	// counters
	OutcolInPkts
	OutcolInPktsPercent
	OutcolInBytes
	OutcolInBytesPercent
	OutcolOutPkts
	OutcolOutPktsPercent
	OutcolOutBytes
	OutcolOutBytesPercent
	OutcolSumPkts
	OutcolSumPktsPercent
	OutcolSumBytes
	OutcolSumBytesPercent
	OutcolBothPktsRcvd
	OutcolBothPktsSent
	OutcolBothPktsPercent
	OutcolBothBytesRcvd
	OutcolBothBytesSent
	OutcolBothBytesPercent
	CountOutcol
)

const (
	packetsStr = "packets"
	bytesStr   = "bytes"
)

// columns returns the list of OutputColumns that (might) be printed.
// timed indicates whether we're supposed to print timestamps. attributes lists
// all attributes we have to print. d tells us which counters to print.
// in this function (and some others) ORDER matters
func columns(selector types.LabelSelector, attributes []types.Attribute, d types.Direction) (cols []OutputColumn) {
	if selector.Timestamp {
		cols = append(cols, OutcolTime)
	}
	// this order represents the hierarchy host > ifaces
	if selector.Hostname {
		cols = append(cols, OutcolHostname)
	}
	if selector.HostID {
		cols = append(cols, OutcolHostID)
	}
	if selector.Iface {
		cols = append(cols, OutcolIface)
	}

	for _, attrib := range attributes {
		switch attrib.Name() {
		case types.SIPName:
			cols = append(cols, OutcolSIP)
		case types.DIPName:
			cols = append(cols, OutcolDIP)
		case types.ProtoName:
			cols = append(cols, OutcolProto)
		case types.DportName:
			cols = append(cols, OutcolDport)
		}
	}

	switch d {
	case types.DirectionIn:
		cols = append(cols,
			OutcolInPkts,
			OutcolInPktsPercent,
			OutcolInBytes,
			OutcolInBytesPercent)
	case types.DirectionOut:
		cols = append(cols,
			OutcolOutPkts,
			OutcolOutPktsPercent,
			OutcolOutBytes,
			OutcolOutBytesPercent)
	case types.DirectionBoth:
		cols = append(cols,
			OutcolBothPktsRcvd,
			OutcolBothPktsSent,
			OutcolBothPktsPercent,
			OutcolBothBytesRcvd,
			OutcolBothBytesSent,
			OutcolBothBytesPercent)
	case types.DirectionSum:
		cols = append(cols,
			OutcolSumPkts,
			OutcolSumPktsPercent,
			OutcolSumBytes,
			OutcolSumBytesPercent)
	}

	return
}

// Formatter provides methods for printing various types/units of values.
// Each output format has an associated Formatter implementation, for instance
// for csv, there is CSVFormatter.
type Formatter interface {
	// Size deals with data sizes (i.e. bytes)
	Size(uint64) string
	Duration(time.Duration) string
	Count(uint64) string
	Float(float64) string
	Time(epoch int64) string
	// String is needed because some formats escape strings
	String(string) string
}

func tryLookup(ips2domains map[string]string, ip string) string {
	if dom, exists := ips2domains[ip]; exists {
		return dom
	}
	return ip
}

// extract extracts the string that needs to be printed for the given OutputColumn.
// The format argument is used to format the string appropriatly for the desired
// output format. ips2domains is needed for reverse DNS lookups. totals is needed
// for percentage calculations. e contains the actual data that is extracted.
func extract(format Formatter, ips2domains map[string]string, totals types.Counters, row Row, col OutputColumn) string {
	nz := func(u uint64) uint64 {
		if u == 0 {
			u = (1 << 64) - 1
		}
		return u
	}

	switch col {
	case OutcolTime:
		return format.Time(row.Labels.Timestamp.Unix())
	case OutcolIface:
		return format.String(row.Labels.Iface)
	case OutcolHostname:
		return format.String(row.Labels.Hostname)
	case OutcolHostID:
		return format.String(row.Labels.HostID)

	case OutcolSIP:
		return format.String(tryLookup(ips2domains, row.Attributes.SrcIP.String()))
	case OutcolDIP:
		return format.String(tryLookup(ips2domains, row.Attributes.DstIP.String()))
	case OutcolDport:
		return format.String(fmt.Sprintf("%d", row.Attributes.DstPort))
	case OutcolProto:
		return format.String(protocols.GetIPProto(int(row.Attributes.IPProto)))

	case OutcolInBytes, OutcolBothBytesRcvd:
		return format.Size(row.Counters.BytesRcvd)
	case OutcolInBytesPercent:
		return format.Float(float64(100*row.Counters.BytesRcvd) / float64(nz(totals.BytesRcvd)))
	case OutcolInPkts, OutcolBothPktsRcvd:
		return format.Count(row.Counters.PacketsRcvd)
	case OutcolInPktsPercent:
		return format.Float(float64(100*row.Counters.PacketsRcvd) / float64(nz(totals.PacketsRcvd)))
	case OutcolOutBytes, OutcolBothBytesSent:
		return format.Size(row.Counters.BytesSent)
	case OutcolOutBytesPercent:
		return format.Float(float64(100*row.Counters.BytesSent) / float64(nz(totals.BytesSent)))
	case OutcolOutPkts, OutcolBothPktsSent:
		return format.Count(row.Counters.PacketsSent)
	case OutcolOutPktsPercent:
		return format.Float(float64(100*row.Counters.PacketsSent) / float64(nz(totals.PacketsSent)))
	case OutcolSumBytes:
		return format.Size(row.Counters.BytesRcvd + row.Counters.BytesSent)
	case OutcolSumBytesPercent, OutcolBothBytesPercent:
		return format.Float(float64(100*(row.Counters.SumBytes())) / float64(nz(totals.SumBytes())))
	case OutcolSumPkts:
		return format.Count(row.Counters.SumPackets())
	case OutcolSumPktsPercent, OutcolBothPktsPercent:
		return format.Float(float64(100*(row.Counters.SumPackets())) / float64(nz(totals.SumPackets())))
	default:
		panic("unknown OutputColumn value")
	}
}

// extractTotal is similar to extract but extracts a total from totals rather
// than an element of an Entry.
func extractTotal(format Formatter, totals types.Counters, col OutputColumn) string {
	switch col {
	case OutcolInBytes, OutcolBothBytesRcvd:
		return format.Size(totals.BytesRcvd)
	case OutcolInPkts, OutcolBothPktsRcvd:
		return format.Count(totals.PacketsRcvd)
	case OutcolOutBytes, OutcolBothBytesSent:
		return format.Size(totals.BytesSent)
	case OutcolOutPkts, OutcolBothPktsSent:
		return format.Count(totals.PacketsSent)
	case OutcolSumBytes:
		return format.Size(totals.SumBytes())
	case OutcolSumPkts:
		return format.Count(totals.SumPackets())
	default:
		panic("unknown or incorrect OutputColumn value")
	}
}

// describe comes up with a nice string for the given SortOrder and types.Direction.
func describe(o SortOrder, d types.Direction) string {
	result := "accumulated "
	switch o {
	case SortPackets:
		result += "packets "
	case SortTraffic:
		result += "data volume "
	case SortTime:
		return "first packet time" // TODO(lob): Is this right?
	}

	switch d {
	case types.DirectionSum, types.DirectionBoth:
		result += "(sent and received)"
	case types.DirectionIn:
		result += "(received only)"
	case types.DirectionOut:
		result += "(sent only)"
	}

	return result
}

// TablePrinter provides an interface for printing output tables in various
// formats, e.g. JSON, CSV, and nicely aligned human readable text.
//
// You will typically want to call AddRow() for each entry you want to print
// (in order). When you've added all rows, you can add a footer or summary with
// Footer. Not all implementations use all the arguments provided to Footer().
// Lastly, you should call Print() to make sure that all data is printed.
//
// Note that some impementations may start printing data before you call Print().
type TablePrinter interface {
	AddRow(row Row) error
	AddRows(ctx context.Context, rows Rows) error
	Footer(ctx context.Context, result *Result) error
	Print(result *Result) error
}

// basePrinter encapsulates variables and methods used by all TablePrinter
// implementations.
type basePrinter struct {
	output io.Writer

	sort SortOrder

	selector types.LabelSelector

	direction types.Direction

	// query attributes
	attributes []types.Attribute

	ips2domains map[string]string

	// needed for computing percentages
	totals types.Counters

	cols []OutputColumn
}

// newBasePrinter sets up the basic printing facilities
func newBasePrinter(
	output io.Writer,
	sort SortOrder,
	selector types.LabelSelector,
	direction types.Direction,
	attributes []types.Attribute,
	ips2domains map[string]string,
	totals types.Counters,
) basePrinter {
	result := basePrinter{output, sort, selector, direction, attributes, ips2domains, totals,
		columns(selector, attributes, direction),
	}

	return result
}

// PrinterConfig configures printer behavior
type PrinterConfig struct {
	Format        string
	SortOrder     SortOrder
	LabelSelector types.LabelSelector
	Direction     types.Direction

	Attributes []types.Attribute
	Totals     types.Counters
	NumFlows   int

	resolutionTimeout time.Duration
	ipDomainMapping   map[string]string

	printQueryStats bool
}

// PrinterOption allows to configure the printer
type PrinterOption func(*PrinterConfig)

// WithIPDomainMapping adds DNS resolution capabilities to the printer
func WithIPDomainMapping(ipDomain map[string]string, resolutionTimeout time.Duration) PrinterOption {
	return func(pc *PrinterConfig) {
		pc.ipDomainMapping = ipDomain
		pc.resolutionTimeout = resolutionTimeout
	}
}

// WithQueryStats sets whether detailed query statistics should be printed in footer
func WithQueryStats(b bool) PrinterOption {
	return func(pc *PrinterConfig) {
		pc.printQueryStats = b
	}
}

// NewTablePrinter instantiates a new table printer
func NewTablePrinter(output io.Writer, cfg *PrinterConfig) (TablePrinter, error) {
	b := newBasePrinter(output, cfg.SortOrder, cfg.LabelSelector, cfg.Direction, cfg.Attributes, cfg.ipDomainMapping, cfg.Totals)

	var printer TablePrinter
	switch cfg.Format {
	case "txt":
		printer = NewTextTablePrinter(b, cfg.NumFlows, cfg.resolutionTimeout, cfg.printQueryStats)
	case "csv":
		printer = NewCSVTablePrinter(b)
	default:
		return nil, fmt.Errorf("unknown output format %s", cfg.Format)
	}
	return printer, nil
}

// CSVFormatter writes lines in CSV format
type CSVFormatter struct{}

// Size prints the integers size
func (CSVFormatter) Size(s uint64) string {
	return fmt.Sprint(s)
}

// Duration prints the string representation of duration
func (CSVFormatter) Duration(d time.Duration) string {
	return fmt.Sprint(d)
}

// Count prints c as string
func (CSVFormatter) Count(c uint64) string {
	return fmt.Sprint(c)
}

// Float string formats f
func (CSVFormatter) Float(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

// Time prints epoch as string
func (CSVFormatter) Time(epoch int64) string {
	return fmt.Sprint(epoch)
}

// String returns s
func (CSVFormatter) String(s string) string {
	return s
}

// CSVTablePrinter writes out all flows in CSV format
type CSVTablePrinter struct {
	basePrinter
	writer *csv.Writer
	fields []string
}

// NewCSVTablePrinter creates a new CSVTablePrinter
func NewCSVTablePrinter(b basePrinter) *CSVTablePrinter {
	c := CSVTablePrinter{
		b,
		csv.NewWriter(b.output),
		make([]string, 0, len(b.cols)),
	}

	headers := append(types.AllColumns(), []string{
		packetsStr, "%", "data vol.", "%",
		packetsStr, "%", "data vol.", "%",
		packetsStr, "%", "data vol.", "%",
		"packets received", "packets sent", "%", "data vol. received", "data vol. sent", "%",
	}...)

	for _, col := range c.cols {
		c.fields = append(c.fields, headers[col])
	}
	// Since these fields are static this should never fail
	if err := c.writer.Write(c.fields); err != nil {
		panic(err)
	}

	return &c
}

// AddRow writes a row to the CSVTablePrinter
func (c *CSVTablePrinter) AddRow(row Row) error {
	c.fields = c.fields[:0]
	for _, col := range c.cols {
		c.fields = append(c.fields, extract(CSVFormatter{}, c.ips2domains, c.totals, row, col))
	}
	return c.writer.Write(c.fields)
}

// AddRows adds several flow entries to the CSVTablePrinter
func (c *CSVTablePrinter) AddRows(ctx context.Context, rows Rows) error {
	return addRows(ctx, c, rows)
}

// Footer appends the CSV footer to the CSVTablePrinter
func (c *CSVTablePrinter) Footer(_ context.Context, result *Result) error {
	var summaryEntries [CountOutcol]string
	summaryEntries[OutcolInPkts] = "Overall packets"
	summaryEntries[OutcolInBytes] = "Overall data volume (bytes)"
	summaryEntries[OutcolOutPkts] = summaryEntries[OutcolInPkts]
	summaryEntries[OutcolOutBytes] = summaryEntries[OutcolInBytes]
	summaryEntries[OutcolSumPkts] = summaryEntries[OutcolInPkts]
	summaryEntries[OutcolSumBytes] = summaryEntries[OutcolInBytes]
	summaryEntries[OutcolBothPktsRcvd] = "Received packets"
	summaryEntries[OutcolBothPktsSent] = "Sent packets"
	summaryEntries[OutcolBothBytesRcvd] = "Received data volume (bytes)"
	summaryEntries[OutcolBothBytesSent] = "Sent data volume (bytes)"
	for _, col := range c.cols {
		if summaryEntries[col] != "" {
			if err := c.writer.Write([]string{summaryEntries[col], extractTotal(CSVFormatter{}, c.totals, col)}); err != nil {
				return err
			}
		}
	}
	if err := c.writer.Write([]string{"Sorting and flow direction", describe(c.sort, c.direction)}); err != nil {
		return err
	}
	return c.writer.Write([]string{"Interface", strings.Join(result.Summary.Interfaces, ",")})
}

// Print flushes the writer and actually prints out all CSV rows contained in the table printer
func (c *CSVTablePrinter) Print(_ *Result) error {
	c.writer.Flush()
	// TODO: adding the host statuses
	return nil
}

// TextFormatter table formats goProbe flows (goQuery's default)
type TextFormatter struct{}

// NewTextFormatter returns a new TextFormatter
func NewTextFormatter() TextFormatter {
	return TextFormatter{}
}

// Size prints out size in a human-readable format (e.g. 10 MB)
func (TextFormatter) Size(size uint64) string {
	return formatting.Size(size)
}

// Duration prints out d in a human-readable duration format
func (TextFormatter) Duration(d time.Duration) string {
	return formatting.Duration(d)
}

// Count prints val in concise human-readable form (e.g. 1 K instead of 1000)
func (TextFormatter) Count(val uint64) string {
	return formatting.Count(val)
}

// Float prints f rounded to two decimals
func (TextFormatter) Float(f float64) string {
	if f == 0 {
		return fmt.Sprintf("%.2f", f)
	}
	return fmt.Sprintf("%.2f", f)
}

// Time formats epoch to "06-01-02 15:04:05"
func (TextFormatter) Time(epoch int64) string {
	return time.Unix(epoch, 0).Format(types.DefaultTimeOutputFormat)
}

// String returns s
func (TextFormatter) String(s string) string {
	return s
}

// TextTablePrinter pretty prints all flows
type TextTablePrinter struct {
	basePrinter
	writer         *tabwriter.Writer
	footerWriter   *FooterTabwriter
	numFlows       int
	resolveTimeout time.Duration
	numPrinted     int

	printQueryStats bool
}

// NewTextTablePrinter creates a new table printer
func NewTextTablePrinter(b basePrinter, numFlows int, resolveTimeout time.Duration, printQueryStats bool) *TextTablePrinter {
	var t = &TextTablePrinter{
		basePrinter:     b,
		writer:          tabwriter.NewWriter(b.output, 0, 1, 2, ' ', tabwriter.AlignRight),
		footerWriter:    NewFooterTabwriter(b.output),
		numFlows:        numFlows,
		resolveTimeout:  resolveTimeout,
		printQueryStats: printQueryStats,
	}

	var header1 [CountOutcol]string
	header1[OutcolInPkts] = packetsStr
	header1[OutcolInBytes] = bytesStr
	header1[OutcolOutPkts] = packetsStr
	header1[OutcolOutBytes] = bytesStr
	header1[OutcolSumPkts] = packetsStr
	header1[OutcolSumBytes] = bytesStr
	header1[OutcolBothPktsRcvd] = packetsStr
	header1[OutcolBothPktsSent] = packetsStr
	header1[OutcolBothBytesRcvd] = bytesStr
	header1[OutcolBothBytesSent] = bytesStr

	var header2 = append(types.AllColumns(), []string{
		"in", "%", "in", "%",
		"out", "%", "out", "%",
		"in+out", "%", "in+out", "%",
		"in", "out", "%", "in", "out", "%",
	}...)

	for _, col := range t.cols {
		fmt.Fprint(t.writer, header1[col])
		fmt.Fprint(t.writer, "\t")

	}
	fmt.Fprintln(t.writer)

	for _, col := range t.cols {
		fmt.Fprint(t.writer, header2[col])
		fmt.Fprint(t.writer, "\t")
	}
	fmt.Fprintln(t.writer)

	return t
}

func addRows(ctx context.Context, p TablePrinter, rows Rows) error {
	for i, row := range rows {
		select {
		case <-ctx.Done():
			// printer filling was cancelled
			return fmt.Errorf("query cancelled before fully filled. %d/%d rows processed", i, len(rows))
		default:
			if err := p.AddRow(row); err != nil {
				return err
			}
		}
	}
	return nil
}

// AddRow adds a flow entry to the table printer
func (t *TextTablePrinter) AddRow(row Row) error {
	for _, col := range t.cols {
		fmt.Fprintf(t.writer, "%s\t", extract(TextFormatter{}, t.ips2domains, t.totals, row, col))
	}
	fmt.Fprintln(t.writer)
	t.numPrinted++
	return nil
}

// AddRows adds several flow entries to the table printer
func (t *TextTablePrinter) AddRows(ctx context.Context, rows Rows) error {
	return addRows(ctx, t, rows)
}

const (
	hostsKey      = "Hosts"
	ifaceKey      = "Interface"
	queryStatsKey = "Query stats"
	sortedByKey   = "Sorted by"
	totalsKey     = "Totals"
	traceIDKey    = "Trace ID"
)

// Footer appends the summary to the table printer
func (t *TextTablePrinter) Footer(ctx context.Context, result *Result) error {
	var isTotal [CountOutcol]bool
	isTotal[OutcolInPkts] = true
	isTotal[OutcolInBytes] = true
	isTotal[OutcolOutPkts] = true
	isTotal[OutcolOutBytes] = true
	isTotal[OutcolSumPkts] = true
	isTotal[OutcolSumBytes] = true
	isTotal[OutcolBothPktsRcvd] = true
	isTotal[OutcolBothPktsSent] = true
	isTotal[OutcolBothBytesRcvd] = true
	isTotal[OutcolBothBytesSent] = true

	// line with ... in the right places to separate totals
	for _, col := range t.cols {
		if isTotal[col] && t.numPrinted < t.numFlows {
			fmt.Fprint(t.writer, "...")
		}
		fmt.Fprint(t.writer, "\t")
	}
	fmt.Fprintln(t.writer)

	// Totals
	for _, col := range t.cols {
		if isTotal[col] {
			fmt.Fprint(t.writer, extractTotal(TextFormatter{}, t.totals, col))
		}
		fmt.Fprint(t.writer, "\t")
	}
	fmt.Fprintln(t.writer)

	if t.direction == types.DirectionBoth {
		for range t.cols[1:] {
			fmt.Fprint(t.writer, "\t")
		}
		fmt.Fprintln(t.writer)

		fmt.Fprint(t.writer, totalsKey+":\t")
		for _, col := range t.cols[1:] {
			if col == OutcolBothPktsSent {
				fmt.Fprint(t.writer, TextFormatter{}.Count(t.totals.SumPackets()))
			}
			if col == OutcolBothBytesSent {
				fmt.Fprint(t.writer, TextFormatter{}.Size(t.totals.SumBytes()))
			}
			fmt.Fprint(t.writer, "\t")
		}
		fmt.Fprintln(t.writer)
	}

	textFormatter := TextFormatter{}

	// Summary
	result.Summary.TimeRange.PrintFooter(t.footerWriter)

	// only print interface names if the attributes don't include the interface
	var (
		ifaceSummary string
		iKey         = ifaceKey
	)
	if len(result.Summary.Interfaces) > 1 {
		iKey += "s"
	}
	if t.basePrinter.selector.Iface {
		ifaceSummary = result.Summary.Interfaces.Summary() + " queried"
	} else {
		ifaceSummary = strings.Join(result.Summary.Interfaces, ",")
	}
	// attach distributed query information from hosts if available
	if len(result.HostsStatuses) > 1 {
		t.footerWriter.WriteEntry(iKey+" / "+hostsKey, ifaceSummary+" on "+result.HostsStatuses.Summary())
	} else {
		t.footerWriter.WriteEntry(ifaceKey, ifaceSummary)
	}

	t.footerWriter.WriteEntry(sortedByKey, describe(t.sort, t.direction))

	result.Query.PrintFooter(t.footerWriter)
	t.footerWriter.WriteEntry(queryStatsKey, "displayed top %s hits out of %s in %s",
		formatting.CountSmall(uint64(result.Summary.Hits.Displayed)),
		formatting.CountSmall(uint64(result.Summary.Hits.Total)),
		textFormatter.Duration(result.Summary.Timings.QueryDuration),
	)

	if t.printQueryStats {
		stats := result.Summary.Stats
		// we leave the key empty on purpose since the displayed info constitutes query statistics
		t.footerWriter.WriteEntry("Bytes loaded", "%s",
			formatting.SizeSmall(stats.BytesLoaded),
		)
		t.footerWriter.WriteEntry("Bytes decompressed", "%s",
			formatting.SizeSmall(stats.BytesDecompressed),
		)
		t.footerWriter.WriteEntry("Blocks processed", "%s",
			formatting.Count(stats.BlocksProcessed),
		)
		t.footerWriter.WriteEntry("Blocks corrupted", "%s",
			formatting.CountSmall(stats.BlocksCorrupted),
		)
		t.footerWriter.WriteEntry("Directories processed", "%s",
			formatting.CountSmall(stats.DirectoriesProcessed),
		)
		t.footerWriter.WriteEntry("Workloads", "%s",
			formatting.CountSmall(stats.Workloads),
		)
	}

	// provide the trace ID in case it was provided in a distributed call
	if len(result.HostsStatuses) > 1 {
		sc := trace.SpanFromContext(ctx).SpanContext()
		if sc.HasTraceID() {
			t.footerWriter.WriteEntry(traceIDKey, sc.TraceID().String())
		}
	}

	return nil
}

// Print flushes the table printer and outputs all entries to stdout
func (t *TextTablePrinter) Print(result *Result) error {
	fmt.Fprintln(t.output) // newline between prompt and results
	if err := t.writer.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(t.output)
	if err := t.footerWriter.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(t.output)
	return nil
}

// PrintDetailedSummary prints additional details in the summary, such as the full interface list or all host statuses
func (t *TextTablePrinter) PrintDetailedSummary(result *Result) error {
	// print the interface list
	fmt.Fprintln(t.output, "Interfaces:")
	err := result.Summary.Interfaces.Print(t.output)
	if err != nil {
		return err
	}

	// print the host list
	if len(result.HostsStatuses) > 1 {
		fmt.Fprintln(t.output, "Host Statuses:")
		err = result.HostsStatuses.Print(t.output)
		if err != nil {
			return err
		}
	}
	return nil
}
