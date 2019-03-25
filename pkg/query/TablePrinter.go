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
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/els0r/goProbe/pkg/goDB"
)

// OutputColumn's domain ranges over all possible output columns.
// Not every format prints every output column, e.g. the InfluxDBTablePrinter
// completely ignores all percentages.
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
	// ANSI_SET_BOLD = "\x1b[1m"
	// ANSI_RESET    = "\x1b[0m"
)

// columns returns the list of OutputColumns that (might) be printed.
// timed indicates whether we're supposed to print timestamps. attributes lists
// all attributes we have to print. d tells us which counters to print.
func columns(hasAttrTime, hasAttrIface bool, attributes []goDB.Attribute, d Direction) (cols []OutputColumn) {
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
	case DirectionIn:
		cols = append(cols,
			OutcolInPkts,
			OutcolInPktsPercent,
			OutcolInBytes,
			OutcolInBytesPercent)
	case DirectionOut:
		cols = append(cols,
			OutcolOutPkts,
			OutcolOutPktsPercent,
			OutcolOutBytes,
			OutcolOutBytesPercent)
	case DirectionBoth:
		cols = append(cols,
			OutcolBothPktsRcvd,
			OutcolBothPktsSent,
			OutcolBothPktsPercent,
			OutcolBothBytesRcvd,
			OutcolBothBytesSent,
			OutcolBothBytesPercent)
	case DirectionSum:
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
	// String is needed because some formats escape strings (e.g. InfluxDB)
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
func extract(format Formatter, ips2domains map[string]string, totals Counts, e Entry, col OutputColumn) string {
	nz := func(u uint64) uint64 {
		if u == 0 {
			u = (1 << 64) - 1
		}
		return u
	}

	switch col {
	case OutcolTime:
		return format.Time(e.k.Time)
	case OutcolIface:
		return format.String(e.k.Iface)

	case OutcolSip:
		ip := goDB.SipAttribute{}.ExtractStrings(&e.k)[0]
		return format.String(tryLookup(ips2domains, ip))
	case OutcolDip:
		ip := goDB.DipAttribute{}.ExtractStrings(&e.k)[0]
		return format.String(tryLookup(ips2domains, ip))
	case OutcolDport:
		return format.String(goDB.DportAttribute{}.ExtractStrings(&e.k)[0])
	case OutcolProto:
		return format.String(goDB.ProtoAttribute{}.ExtractStrings(&e.k)[0])

	case OutcolInBytes, OutcolBothBytesRcvd:
		return format.Size(e.nBr)
	case OutcolInBytesPercent:
		return format.Float(float64(100*e.nBr) / float64(nz(totals.BytesRcvd)))
	case OutcolInPkts, OutcolBothPktsRcvd:
		return format.Count(e.nPr)
	case OutcolInPktsPercent:
		return format.Float(float64(100*e.nPr) / float64(nz(totals.PktsRcvd)))
	case OutcolOutBytes, OutcolBothBytesSent:
		return format.Size(e.nBs)
	case OutcolOutBytesPercent:
		return format.Float(float64(100*e.nBs) / float64(nz(totals.BytesSent)))
	case OutcolOutPkts, OutcolBothPktsSent:
		return format.Count(e.nPs)
	case OutcolOutPktsPercent:
		return format.Float(float64(100*e.nPs) / float64(nz(totals.PktsSent)))
	case OutcolSumBytes:
		return format.Size(e.nBr + e.nBs)
	case OutcolSumBytesPercent, OutcolBothBytesPercent:
		return format.Float(float64(100*(e.nBr+e.nBs)) / float64(nz(totals.BytesRcvd+totals.BytesSent)))
	case OutcolSumPkts:
		return format.Count(e.nPr + e.nPs)
	case OutcolSumPktsPercent, OutcolBothPktsPercent:
		return format.Float(float64(100*(e.nPr+e.nPs)) / float64(nz(totals.PktsRcvd+totals.PktsSent)))
	default:
		panic("unknown OutputColumn value")
	}
}

// extractTotal is similar to extract but extracts a total from totals rather
// than an element of an Entry.
func extractTotal(format Formatter, totals Counts, col OutputColumn) string {
	switch col {
	case OutcolInBytes, OutcolBothBytesRcvd:
		return format.Size(totals.BytesRcvd)
	case OutcolInPkts, OutcolBothPktsRcvd:
		return format.Count(totals.PktsRcvd)
	case OutcolOutBytes, OutcolBothBytesSent:
		return format.Size(totals.BytesSent)
	case OutcolOutPkts, OutcolBothPktsSent:
		return format.Count(totals.PktsSent)
	case OutcolSumBytes:
		return format.Size(totals.BytesRcvd + totals.BytesSent)
	case OutcolSumPkts:
		return format.Count(totals.PktsRcvd + totals.PktsSent)
	default:
		panic("unknown or incorrect OutputColumn value")
	}
}

// describe comes up with a nice string for the given SortOrder and Direction.
func describe(o SortOrder, d Direction) string {
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
	case DirectionSum, DirectionBoth:
		result += "(sent and received)"
	case DirectionIn:
		result += "(received only)"
	case DirectionOut:
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
	AddRow(entry Entry)
	Footer(conditional string, spanFirst, spanLast time.Time, queryDuration, resolveDuration time.Duration)
	Print() error
}

// basePrinter encapsulates variables and methods used by all TablePrinter
// implementations.
type basePrinter struct {
	output io.Writer

	sort SortOrder

	hasAttrTime, hasAttrIface bool

	direction Direction

	// query attributes
	attributes []goDB.Attribute

	ips2domains map[string]string

	// needed for computing percentages
	totals Counts

	ifaces string

	cols []OutputColumn
}

func makeBasePrinter(
	output io.Writer,
	sort SortOrder,
	hasAttrTime, hasAttrIface bool,
	direction Direction,
	attributes []goDB.Attribute,
	ips2domains map[string]string,
	totalInPkts, totalOutPkts, totalInBytes, totalOutBytes uint64,
	ifaces string,
) basePrinter {
	result := basePrinter{
		output,
		sort,
		hasAttrTime, hasAttrIface,
		direction,
		attributes,
		ips2domains,
		Counts{totalInPkts, totalOutPkts, totalInBytes, totalOutBytes},
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
func (c *CSVTablePrinter) AddRow(entry Entry) {
	c.fields = c.fields[:0]
	for _, col := range c.cols {
		c.fields = append(c.fields, extract(CSVFormatter{}, c.ips2domains, c.totals, entry, col))
	}
	c.writer.Write(c.fields)
}

// Footer appends the CSV footer to the table
func (c *CSVTablePrinter) Footer(conditional string, spanFirst, spanLast time.Time, queryDuration, resolveDuration time.Duration) {
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
	result, _ := json.Marshal(s)
	return string(result)
}

// Duration marshals d into a JSON string
func (JSONFormatter) Duration(d time.Duration) string {
	result, _ := json.Marshal(d)
	return string(result)
}

// Count marshals c into a JSON string
func (JSONFormatter) Count(c uint64) string {
	result, _ := json.Marshal(c)
	return string(result)
}

// Float marshals f into a JSON string
func (JSONFormatter) Float(f float64) string {
	result, _ := json.Marshal(f)
	return string(result)
}

// Time marshals epoch into a JSON string
func (JSONFormatter) Time(epoch int64) string {
	// convert to string first for legacy reasons
	result, _ := json.Marshal(fmt.Sprint(epoch))
	return string(result)
}

// String marshals s into a JSON string
func (JSONFormatter) String(s string) string {
	result, _ := json.Marshal(s)
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
	rows      []map[string]*json.RawMessage
	data      map[string]interface{}
	queryType string
}

// NewJSONTablePrinter creates a new JSONTablePrinter
func NewJSONTablePrinter(b basePrinter, queryType string) *JSONTablePrinter {
	j := JSONTablePrinter{
		b,
		nil,
		make(map[string]interface{}),
		queryType,
	}

	return &j
}

// AddRow adds a new JSON formatted row to the JSON printer
func (j *JSONTablePrinter) AddRow(entry Entry) {
	row := make(map[string]*json.RawMessage)
	for _, col := range j.cols {
		val := json.RawMessage(extract(JSONFormatter{}, j.ips2domains, j.totals, entry, col))
		row[jsonKeys[col]] = &val
	}
	j.rows = append(j.rows, row)
}

// Footer adds the summary footer in JSON format
func (j *JSONTablePrinter) Footer(conditional string, spanFirst, spanLast time.Time, queryDuration, resolveDuration time.Duration) {
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
			val := json.RawMessage(extractTotal(JSONFormatter{}, j.totals, col))
			summary[summaryEntries[col]] = &val
		}
	}

	j.data["summary"] = summary
}

// Print prints out the JSON formatted flows to stdout
func (j *JSONTablePrinter) Print() error {
	j.data[j.queryType] = j.rows
	return json.NewEncoder(j.output).Encode(j.data)
}

// TextFormatter table formats goProbe flows (goQuery's default)
type TextFormatter struct{}

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

	for val > 1000 {
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
	return time.Unix(epoch, 0).Format("06-01-02 15:04:05")
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
func (t *TextTablePrinter) AddRow(entry Entry) {
	for _, col := range t.cols {
		fmt.Fprint(t.writer, extract(TextFormatter{}, t.ips2domains, t.totals, entry, col))
		fmt.Fprint(t.writer, "\t")
	}
	fmt.Fprintln(t.writer)
	t.numPrinted++
}

// Footer appends the summary to the table printer
func (t *TextTablePrinter) Footer(conditional string, spanFirst, spanLast time.Time, queryDuration, resolveDuration time.Duration) {
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

	if t.direction == DirectionBoth {
		for range t.cols[1:] {
			fmt.Fprint(t.writer, "\t")
		}
		fmt.Fprintln(t.writer)

		fmt.Fprint(t.writer, "Totals:\t")
		for _, col := range t.cols[1:] {
			if col == OutcolBothPktsSent {
				fmt.Fprint(t.writer, TextFormatter{}.Count(t.totals.PktsRcvd+t.totals.PktsSent))
			}
			if col == OutcolBothBytesSent {
				fmt.Fprint(t.writer, TextFormatter{}.Size(t.totals.BytesRcvd+t.totals.BytesSent))
			}
			fmt.Fprint(t.writer, "\t")
		}
		fmt.Fprintln(t.writer)
	}

	// Summary
	fmt.Fprintf(t.footwriter, "Timespan / Interface\t: [%s, %s] / %s\n",
		spanFirst.Format("2006-01-02 15:04:05"),
		spanLast.Format("2006-01-02 15:04:05"),
		t.ifaces)
	fmt.Fprintf(t.footwriter, "Sorted by\t: %s\n",
		describe(t.sort, t.direction))
	if resolveDuration > 0 {
		fmt.Fprintf(t.footwriter, "Reverse DNS stats\t: RDNS took %s, timeout was %s\n",
			TextFormatter{}.Duration(resolveDuration),
			TextFormatter{}.Duration(t.resolveTimeout))
	}
	fmt.Fprintf(t.footwriter, "Query stats\t: %s hits in %s\n",
		TextFormatter{}.Count(uint64(t.numFlows)),
		TextFormatter{}.Duration(queryDuration))
	if conditional != "" {
		fmt.Fprintf(t.footwriter, "Conditions:\t: %s\n",
			conditional)
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

// The term 'key' has two different meanings in the InfluxDB documentation.
// Here we mean key as in "the key field of a protocol line '[key] [fields] [timestamp]'".
const influxDBKey = "goprobe_flows"

// The term 'key' has two different meanings in the InfluxDB documentation.
// Here we mean key as in "key-value metric". Key-value metrics are needed for
// specifying tags and fields.
var influxDBKeys = [CountOutcol]string{
	"", // not used
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

// InfluxDBFormatter formats goProbe flows for ingestion into InfluxDB
// See https://docs.influxdata.com/influxdb/v0.10/write_protocols/line/
// for details on the InfluxDB line protocol.
// Important detail: InfluxDB  wants integral values to have the suffix 'i'.
// Floats have no suffix.
type InfluxDBFormatter struct{}

// Size formats s for InfluxDB
func (InfluxDBFormatter) Size(s uint64) string {
	return fmt.Sprintf("%di", s)
}

// Duration formats d for InfluxDB
func (InfluxDBFormatter) Duration(d time.Duration) string {
	return fmt.Sprintf("%di", d.Nanoseconds())
}

// Count formats c for InfluxDB
func (InfluxDBFormatter) Count(c uint64) string {
	return fmt.Sprintf("%di", c)
}

// Float formats f for InfluxDB
func (InfluxDBFormatter) Float(f float64) string {
	return fmt.Sprint(f)
}

// Time formats epoch for InfluxDB
func (InfluxDBFormatter) Time(epoch int64) string {
	// InfluxDB prefers nanosecond epoch timestamps
	return fmt.Sprint(epoch * int64(time.Second))
}

// String formats s for InfluxDB
// Limitation: Since we only use strings in tags, we only escape strings for the
// tag format, not for the field format.
func (InfluxDBFormatter) String(s string) string {
	result := make([]rune, 0, len(s))

	// Escape backslashes and commas
	for _, c := range s {
		switch c {
		case '\\', ',', ' ':
			result = append(result, '\\', c)
		default:
			result = append(result, c)
		}
	}

	return string(result)
}

// InfluxDBTablePrinter prints all flow entries for InfluxDB
type InfluxDBTablePrinter struct {
	basePrinter
	tagCols, fieldCols []OutputColumn
}

// ByInfluxDBKey implements sort.Interface for []OutputColumn
type ByInfluxDBKey []OutputColumn

// Len returns and InfluxDBKey length
func (xs ByInfluxDBKey) Len() int {
	return len(xs)
}

// Less compares InfluxDBKeys
func (xs ByInfluxDBKey) Less(i, j int) bool {
	return bytes.Compare([]byte(influxDBKeys[xs[i]]), []byte(influxDBKeys[xs[j]])) < 0
}

// Swap swaps InfluxDBKeys
func (xs ByInfluxDBKey) Swap(i, j int) {
	xs[i], xs[j] = xs[j], xs[i]
}

// NewInfluxDBTablePrinter creates a new InfluxDB printer
func NewInfluxDBTablePrinter(b basePrinter) *InfluxDBTablePrinter {
	var isTagCol, isFieldCol [CountOutcol]bool
	// OutcolTime is no tag and no field
	isTagCol[OutcolIface] = true
	isFieldCol[OutcolSip] = true
	isFieldCol[OutcolDip] = true
	isFieldCol[OutcolDport] = true
	isTagCol[OutcolProto] = true
	isFieldCol[OutcolInPkts] = true
	// ignore OutcolInPktsPercent
	isFieldCol[OutcolInBytes] = true
	// ignore OutcolInBytesPercent
	isFieldCol[OutcolOutPkts] = true
	// ignore OutcolOutPktsPercent
	isFieldCol[OutcolOutBytes] = true
	// ignore OutcolOutBytesPercent
	isFieldCol[OutcolSumPkts] = true
	// ignore OutcolSumPktsPercent
	isFieldCol[OutcolSumBytes] = true
	// ignore OutcolSumBytesPercent
	isFieldCol[OutcolBothPktsRcvd] = true
	isFieldCol[OutcolBothPktsSent] = true
	// ignore OutcolBothPktsPercent
	isFieldCol[OutcolBothBytesRcvd] = true
	isFieldCol[OutcolBothBytesSent] = true
	// ignore OutcolBothBytesPercent

	var tagCols, fieldCols []OutputColumn

	for _, col := range b.cols {
		if isTagCol[col] {
			tagCols = append(tagCols, col)
		}
		if isFieldCol[col] {
			fieldCols = append(fieldCols, col)
		}
	}

	// influx db documentation: "Tags should be sorted by key before being
	// sent for best performance. The sort should match that from the Go
	// bytes.Compare"
	sort.Sort(ByInfluxDBKey(tagCols))

	var i = &InfluxDBTablePrinter{
		b,
		tagCols, fieldCols,
	}

	return i
}

// AddRow adds a flow entry to the InfluxDBTablePrinter
func (i *InfluxDBTablePrinter) AddRow(entry Entry) {
	// Key + tags
	fmt.Fprint(i.output, influxDBKey)
	for _, col := range i.tagCols {
		fmt.Fprint(i.output, ",")
		fmt.Fprint(i.output, influxDBKeys[col])
		fmt.Fprint(i.output, "=")
		fmt.Fprint(i.output, extract(TextFormatter{}, i.ips2domains, i.totals, entry, col))
	}

	fmt.Fprint(i.output, " ")

	// Fields
	fmt.Fprint(i.output, influxDBKeys[i.fieldCols[0]])
	fmt.Fprint(i.output, "=")
	fmt.Fprint(i.output, extract(InfluxDBFormatter{}, i.ips2domains, i.totals, entry, i.fieldCols[0]))
	for _, col := range i.fieldCols[1:] {
		fmt.Fprint(i.output, ",")
		fmt.Fprint(i.output, influxDBKeys[col])
		fmt.Fprint(i.output, "=")
		fmt.Fprint(i.output, extract(InfluxDBFormatter{}, i.ips2domains, i.totals, entry, col))
	}

	// Time
	if i.hasAttrTime {
		fmt.Fprint(i.output, " ")
		fmt.Fprint(i.output, extract(InfluxDBFormatter{}, i.ips2domains, i.totals, entry, OutcolTime))
	}

	fmt.Fprintln(i.output)
}

// Footer is a no-op for the InfluxDBTablePrinter
func (*InfluxDBTablePrinter) Footer(conditional string, spanFirst, spanLast time.Time, queryDuration, resolveDuration time.Duration) {
	return
}

// Print is a no-op for the InfluxDBTablePrinter
func (*InfluxDBTablePrinter) Print() error {
	return nil
}

// NewTablePrinter provides a convenient interface for instantiating the various
// TablePrinters. You could call it a factory method.
func (s *Statement) NewTablePrinter(ips2domains map[string]string, sums Counts, numFlows int) (TablePrinter, error) {

	b := makeBasePrinter(
		s.Output,
		s.SortBy,
		s.HasAttrTime, s.HasAttrIface,
		s.Direction,
		s.Query.Attributes,
		ips2domains,
		sums.PktsRcvd, sums.PktsSent, sums.BytesRcvd, sums.BytesSent,
		strings.Join(s.Ifaces, ","),
	)

	switch s.Format {
	case "txt":
		return NewTextTablePrinter(b, numFlows, s.ResolveTimeout), nil
	case "json":
		return NewJSONTablePrinter(b, s.QueryType), nil
	case "csv":
		return NewCSVTablePrinter(b), nil
	case "influxdb":
		return NewInfluxDBTablePrinter(b), nil
	default:
		return nil, fmt.Errorf("Unknown output format %s", s.Format)
	}
}

// ErrorMsgMExternal stores a status and message for external callers
type ErrorMsgExternal struct {
	Status  string `json:"status"`
	Message string `json:"statusMessage"`
}
