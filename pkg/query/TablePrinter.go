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

package query

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/protocols"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
)

// OutputColumn ranges over all possible output columns.
// Not every format prints every output column, e.g. the InfluxDBTablePrinter
// completely ignorowall percentages.
type OutputColumn int

// Enumeration of all possible output columns
const (
	OutcolTime OutputColumn = iota
	OutcolIface
	OutcolSip
	OutcolDip
	OutcolDport
	OutcolProto
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

// columns returns the list of OutputColumns that (might) be printed.
// timed indicates whether we're supposed to print timestamps. attributes lists
// all attributes we have to print. d tells us which counters to print.
func columns(hasAttrTime, hasAttrIface bool, attributes []goDB.Attribute, d types.Direction) (cols []OutputColumn) {
	if hasAttrTime {
		cols = append(cols, OutcolTime)
	}

	if hasAttrIface {
		cols = append(cols, OutcolIface)
	}

	for _, attrib := range attributes {
		switch attrib.Name() {
		case "sip":
			cols = append(cols, OutcolSip)
		case "dip":
			cols = append(cols, OutcolDip)
		case "proto":
			cols = append(cols, OutcolProto)
		case "dport":
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
func extract(format Formatter, ips2domains map[string]string, totals results.Counters, row results.Row, col OutputColumn) string {
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

	case OutcolSip:
		return format.String(tryLookup(ips2domains, row.Attributes.SrcIP.String()))
	case OutcolDip:
		return format.String(tryLookup(ips2domains, row.Attributes.DstIP.String()))
	case OutcolDport:
		return format.String(fmt.Sprintf("%d", row.Attributes.DstPort))
	case OutcolProto:
		return format.String(protocols.GetIPProto(int(row.Attributes.IPProto)))

	case OutcolInBytes, OutcolBothBytesRcvd:
		return format.Size(row.Counters.BytesReceived)
	case OutcolInBytesPercent:
		return format.Float(float64(100*row.Counters.BytesReceived) / float64(nz(totals.BytesReceived)))
	case OutcolInPkts, OutcolBothPktsRcvd:
		return format.Count(row.Counters.PacketsReceived)
	case OutcolInPktsPercent:
		return format.Float(float64(100*row.Counters.PacketsReceived) / float64(nz(totals.PacketsReceived)))
	case OutcolOutBytes, OutcolBothBytesSent:
		return format.Size(row.Counters.BytesSent)
	case OutcolOutBytesPercent:
		return format.Float(float64(100*row.Counters.BytesSent) / float64(nz(totals.BytesSent)))
	case OutcolOutPkts, OutcolBothPktsSent:
		return format.Count(row.Counters.PacketsSent)
	case OutcolOutPktsPercent:
		return format.Float(float64(100*row.Counters.PacketsSent) / float64(nz(totals.PacketsSent)))
	case OutcolSumBytes:
		return format.Size(row.Counters.BytesReceived + row.Counters.BytesSent)
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
func extractTotal(format Formatter, totals results.Counters, col OutputColumn) string {
	switch col {
	case OutcolInBytes, OutcolBothBytesRcvd:
		return format.Size(totals.BytesReceived)
	case OutcolInPkts, OutcolBothPktsRcvd:
		return format.Count(totals.PacketsReceived)
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

// describe comes up with a nice string for the given results.SortOrder and types.Direction.
func describe(o results.SortOrder, d types.Direction) string {
	result := "accumulated "
	switch o {
	case results.SortPackets:
		result += "packets "
	case results.SortTraffic:
		result += "data volume "
	case results.SortTime:
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
	AddRow(row results.Row)
	AddRows(ctx context.Context, rows results.Rows) error
	Footer(result *results.Result)
	Print() error
}

// basePrinter encapsulates variables and methods used by all TablePrinter
// implementations.
type basePrinter struct {
	output io.Writer

	sort results.SortOrder

	hasAttrTime, hasAttrIface bool

	direction types.Direction

	// query attributes
	attributes []goDB.Attribute

	ips2domains map[string]string

	// needed for computing percentages
	totals results.Counters

	ifaces string

	cols []OutputColumn
}

func makeBasePrinter(
	output io.Writer,
	sort results.SortOrder,
	hasAttrTime, hasAttrIface bool,
	direction types.Direction,
	attributes []goDB.Attribute,
	ips2domains map[string]string,
	totals results.Counters,
	ifaces string,
) basePrinter {
	result := basePrinter{
		output,
		sort,
		hasAttrTime, hasAttrIface,
		direction,
		attributes,
		ips2domains,
		totals,
		ifaces,
		columns(hasAttrTime, hasAttrIface, attributes, direction),
	}

	return result
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

	headers := [CountOutcol]string{
		"time",
		"iface",
		"sip",
		"dip",
		"dport",
		"proto",
		"packets", "%", "data vol.", "%",
		"packets", "%", "data vol.", "%",
		"packets", "%", "data vol.", "%",
		"packets received", "packets sent", "%", "data vol. received", "data vol. sent", "%",
	}

	for _, col := range c.cols {
		c.fields = append(c.fields, headers[col])
	}
	c.writer.Write(c.fields)

	return &c
}

// AddRow writes a row to the CSVTablePrinter
func (c *CSVTablePrinter) AddRow(row results.Row) {
	c.fields = c.fields[:0]
	for _, col := range c.cols {
		c.fields = append(c.fields, extract(CSVFormatter{}, c.ips2domains, c.totals, row, col))
	}
	c.writer.Write(c.fields)
}

func (c *CSVTablePrinter) AddRows(ctx context.Context, rows results.Rows) error {
	return addRows(ctx, c, rows)
}

// Footer appends the CSV footer to the table
func (c *CSVTablePrinter) Footer(result *results.Result) {
	var summaryEntries [CountOutcol]string
	summaryEntries[OutcolInPkts] = "Overall packets"
	summaryEntries[OutcolInBytes] = "Overall data volume (bytes)"
	summaryEntries[OutcolOutPkts] = "Overall packets"
	summaryEntries[OutcolOutBytes] = "Overall data volume (bytes)"
	summaryEntries[OutcolSumPkts] = "Overall packets"
	summaryEntries[OutcolSumBytes] = "Overall data volume (bytes)"
	summaryEntries[OutcolBothPktsRcvd] = "Received packets"
	summaryEntries[OutcolBothPktsSent] = "Sent packets"
	summaryEntries[OutcolBothBytesRcvd] = "Received data volume (bytes)"
	summaryEntries[OutcolBothBytesSent] = "Sent data volume (bytes)"
	for _, col := range c.cols {
		if summaryEntries[col] != "" {
			c.writer.Write([]string{summaryEntries[col], extractTotal(CSVFormatter{}, c.totals, col)})
		}
	}
	c.writer.Write([]string{"Sorting and flow direction", describe(c.sort, c.direction)})
	c.writer.Write([]string{"Interface", c.ifaces})
}

// Print flushes the writer and actually prints out all CSV rows contained in the table printer
func (c *CSVTablePrinter) Print() error {
	c.writer.Flush()
	return nil
}

// JSONFormatter writes flows in JSON
type JSONFormatter struct{}

// Size marshals s into a JSON string
func (JSONFormatter) Size(s uint64) string {
	result, _ := jsoniter.Marshal(s)
	return string(result)
}

// Duration marshals d into a JSON string
func (JSONFormatter) Duration(d time.Duration) string {
	result, _ := jsoniter.Marshal(d)
	return string(result)
}

// Count marshals c into a JSON string
func (JSONFormatter) Count(c uint64) string {
	result, _ := jsoniter.Marshal(c)
	return string(result)
}

// Float marshals f into a JSON string
func (JSONFormatter) Float(f float64) string {
	result, _ := jsoniter.Marshal(f)
	return string(result)
}

// Time marshals epoch into a JSON string
func (JSONFormatter) Time(epoch int64) string {
	// convert to string first for legacy reasons
	result, _ := jsoniter.Marshal(fmt.Sprint(epoch))
	return string(result)
}

// String marshals s into a JSON string
func (JSONFormatter) String(s string) string {
	result, _ := jsoniter.Marshal(s)
	return string(result)
}

var jsonKeys = [CountOutcol]string{
	"time",
	"iface",
	"sip",
	"dip",
	"dport",
	"proto",
	"packets", "packets_percent", "bytes", "bytes_percent",
	"packets", "packets_percent", "bytes", "bytes_percent",
	"packets", "packets_percent", "bytes", "bytes_percent",
	"packets_rcvd", "packets_sent", "packets_percent", "bytes_rcvd", "bytes_sent", "bytes_percent",
}

// JSONTablePrinter stores all flows as JSON objects and prints them to stdout
type JSONTablePrinter struct {
	basePrinter
	rows      []map[string]*jsoniter.RawMessage
	data      map[string]interface{}
	flat      bool
	queryType string
}

// NewJSONTablePrinter creates a new JSONTablePrinter
func NewJSONTablePrinter(b basePrinter, queryType string, flat bool) *JSONTablePrinter {
	return &JSONTablePrinter{basePrinter: b,
		rows:      nil,
		data:      make(map[string]interface{}),
		flat:      flat,
		queryType: queryType,
	}
}

// AddRow adds a new JSON formatted row to the JSON printer
func (j *JSONTablePrinter) AddRow(row results.Row) {
	r := make(map[string]*jsoniter.RawMessage)
	for _, col := range j.cols {
		val := jsoniter.RawMessage(extract(JSONFormatter{}, j.ips2domains, j.totals, row, col))
		r[jsonKeys[col]] = &val
	}
	j.rows = append(j.rows, r)
}

func addRows(ctx context.Context, p TablePrinter, rows results.Rows) error {
	for _, result := range rows {
		select {
		case <-ctx.Done():
			// printer filling was cancelled
			return fmt.Errorf("context cancelled before fully filled")
		default:
			p.AddRow(result)
		}
	}
	return nil
}

func (j *JSONTablePrinter) AddRows(ctx context.Context, rows results.Rows) error {
	if j.flat {
		j.data[j.queryType] = rows
		return nil
	}
	return addRows(ctx, j, rows)
}

// Footer adds the summary footer in JSON format
func (j *JSONTablePrinter) Footer(result *results.Result) {
	j.data["status"] = "ok"
	j.data["ext_ips"] = externalIPs()

	summary := map[string]interface{}{
		"interface": j.ifaces,
	}
	var summaryEntries [CountOutcol]string
	summaryEntries[OutcolInPkts] = "total_packets"
	summaryEntries[OutcolInBytes] = "total_bytes"
	summaryEntries[OutcolOutPkts] = "total_packets"
	summaryEntries[OutcolOutBytes] = "total_bytes"
	summaryEntries[OutcolSumPkts] = "total_packets"
	summaryEntries[OutcolSumBytes] = "total_bytes"
	summaryEntries[OutcolBothPktsRcvd] = "total_packets_rcvd"
	summaryEntries[OutcolBothPktsSent] = "total_packets_sent"
	summaryEntries[OutcolBothBytesRcvd] = "total_bytes_rcvd"
	summaryEntries[OutcolBothBytesSent] = "total_bytes_sent"
	for _, col := range j.cols {
		if summaryEntries[col] != "" {
			val := jsoniter.RawMessage(extractTotal(JSONFormatter{}, j.totals, col))
			summary[summaryEntries[col]] = &val
		}
	}

	j.data["summary"] = summary
}

// Print prints out the JSON formatted flows to stdout
func (j *JSONTablePrinter) Print() error {
	if !j.flat {
		j.data[j.queryType] = j.rows
	}
	return jsoniter.NewEncoder(j.output).Encode(j.data)
}

// TextFormatter table formats goProbe flows (goQuery's default)
type TextFormatter struct{}

// NewTextFormatter returns a new TextFormatter
func NewTextFormatter() TextFormatter {
	return TextFormatter{}
}

// Size prints out size in a human-readable format (e.g. 10 MB)
func (TextFormatter) Size(size uint64) string {
	count := 0
	var sizeF = float64(size)

	units := []string{" B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}

	for size > 1024 {
		size /= 1024
		sizeF /= 1024.0
		count++
	}
	if sizeF == 0 {
		return fmt.Sprintf("%.2f %s", sizeF, units[count])
	}

	return fmt.Sprintf("%.2f %s", sizeF, units[count])
}

// Duration prints out d in a human-readable duration format
func (TextFormatter) Duration(d time.Duration) string {
	if d/time.Hour != 0 {
		return fmt.Sprintf("%dh%2dm", d/time.Hour, d%time.Hour/time.Minute)
	}
	if d/time.Minute != 0 {
		return fmt.Sprintf("%dm%2ds", d/time.Minute, d%time.Minute/time.Second)
	}
	if d/time.Second != 0 {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dms", d/time.Millisecond)
}

// Count prints val in concise human-readable form (e.g. 1 K instead of 1000)
func (TextFormatter) Count(val uint64) string {
	count := 0
	var valF = float64(val)

	units := []string{" ", "k", "M", "G", "T", "P", "E", "Z", "Y"}

	for val >= 1000 {
		val /= 1000
		valF /= 1000.0
		count++
	}
	if valF == 0 {
		return fmt.Sprintf("%.2f %s", valF, units[count])
	}

	return fmt.Sprintf("%.2f %s", valF, units[count])
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
	return time.Unix(epoch, 0).Format(DefaultTimeOutputFormat)
}

// String returns s
func (TextFormatter) String(s string) string {
	return s
}

// TextTablePrinter pretty prints all flows
type TextTablePrinter struct {
	basePrinter
	writer         *tabwriter.Writer
	footwriter     *tabwriter.Writer
	numFlows       int
	resolveTimeout time.Duration
	numPrinted     int
}

// NewTextTablePrinter creates a new table printer
func NewTextTablePrinter(b basePrinter, numFlows int, resolveTimeout time.Duration) *TextTablePrinter {
	var t = &TextTablePrinter{
		b,
		tabwriter.NewWriter(b.output, 0, 1, 2, ' ', tabwriter.AlignRight),
		tabwriter.NewWriter(b.output, 0, 4, 1, ' ', 0),
		numFlows,
		resolveTimeout,
		0,
	}

	var header1 [CountOutcol]string
	header1[OutcolInPkts] = "packets"
	header1[OutcolInBytes] = "bytes"
	header1[OutcolOutPkts] = "packets"
	header1[OutcolOutBytes] = "bytes"
	header1[OutcolSumPkts] = "packets"
	header1[OutcolSumBytes] = "bytes"
	header1[OutcolBothPktsRcvd] = "packets"
	header1[OutcolBothPktsSent] = "packets"
	header1[OutcolBothBytesRcvd] = "bytes"
	header1[OutcolBothBytesSent] = "bytes"

	var header2 = [CountOutcol]string{
		"time",
		"iface",
		"sip",
		"dip",
		"dport",
		"proto",
		"in", "%", "in", "%",
		"out", "%", "out", "%",
		"in+out", "%", "in+out", "%",
		"in", "out", "%", "in", "out", "%",
	}

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

// AddRow adds a flow entry to the table printer
func (t *TextTablePrinter) AddRow(row results.Row) {
	for _, col := range t.cols {
		fmt.Fprintf(t.writer, "%s\t", extract(TextFormatter{}, t.ips2domains, t.totals, row, col))
	}
	fmt.Fprintln(t.writer)
	t.numPrinted++
}

func (t *TextTablePrinter) AddRows(ctx context.Context, rows results.Rows) error {
	return addRows(ctx, t, rows)
}

// Footer appends the summary to the table printer
func (t *TextTablePrinter) Footer(result *results.Result) {
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

		fmt.Fprint(t.writer, "Totals:\t")
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

	// Summary
	fmt.Fprintf(t.footwriter, "Timespan / Interface\t: [%s, %s] / %s\n",
		result.Summary.TimeFirst.Format(DefaultTimeOutputFormat),
		result.Summary.TimeLast.Format(DefaultTimeOutputFormat),
		strings.Join(result.Summary.Interfaces, ","))
	fmt.Fprintf(t.footwriter, "Sorted by\t: %s\n",
		describe(t.sort, t.direction))
	if result.Summary.Timings.ResolutionDuration > 0 {
		fmt.Fprintf(t.footwriter, "Reverse DNS stats\t: RDNS took %s, timeout was %s\n",
			TextFormatter{}.Duration(result.Summary.Timings.ResolutionDuration),
			TextFormatter{}.Duration(t.resolveTimeout))
	}
	fmt.Fprintf(t.footwriter, "Query stats\t: displayed top %s hits out of %s in %s\n",
		strings.TrimSpace(TextFormatter{}.Count(uint64(result.Summary.Hits.Displayed))),
		TextFormatter{}.Count(uint64(result.Summary.Hits.Total)),
		TextFormatter{}.Duration(result.Summary.Timings.QueryDuration))
	if result.Query.Condition != "" {
		fmt.Fprintf(t.footwriter, "Conditions:\t: %s\n",
			result.Query.Condition)
	}
}

// Print flushes the table printer and outputs all entries to stdout
func (t *TextTablePrinter) Print() error {
	fmt.Fprintln(t.output) // newline between prompt and results
	t.writer.Flush()
	fmt.Fprintln(t.output)
	t.footwriter.Flush()
	fmt.Fprintln(t.output)

	return nil
}

// ErrorMsgExternal storowa status and message for external callers
type ErrorMsgExternal struct {
	Status  string `json:"status"`
	Message string `json:"statusMessage"`
}
