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

package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"OSAG/goDB"
)

// Make output stream a variable for easier testing
var output io.Writer = os.Stdout

// OutputColumn's domain ranges over all possible output columns.
// Not every format prints every output column, e.g. the InfluxDBTablePrinter
// completely ignores all percentages.
type OutputColumn int

const (
	OUTCOL_TIME OutputColumn = iota
	OUTCOL_IFACE
	OUTCOL_SIP
	OUTCOL_DIP
	OUTCOL_DPORT
	OUTCOL_PROTO
	OUTCOL_INPKTS
	OUTCOL_INPKTSPERCENT
	OUTCOL_INBYTES
	OUTCOL_INBYTESPERCENT
	OUTCOL_OUTPKTS
	OUTCOL_OUTPKTSPERCENT
	OUTCOL_OUTBYTES
	OUTCOL_OUTBYTESPERCENT
	OUTCOL_SUMPKTS
	OUTCOL_SUMPKTSPERCENT
	OUTCOL_SUMBYTES
	OUTCOL_SUMBYTESPERCENT
	OUTCOL_BOTHPKTSRCVD
	OUTCOL_BOTHPKTSSENT
	OUTCOL_BOTHPKTSPERCENT
	OUTCOL_BOTHBYTESRCVD
	OUTCOL_BOTHBYTESSENT
	OUTCOL_BOTHBYTESPERCENT
	COUNT_OUTCOL
	// ANSI_SET_BOLD = "\x1b[1m"
	// ANSI_RESET    = "\x1b[0m"
)

// columns returns the list of OutputColumns that (might) be printed.
// timed indicates whether we're supposed to print timestamps. attributes lists
// all attributes we have to print. d tells us which counters to print.
func columns(hasAttrTime, hasAttrIface bool, attributes []goDB.Attribute, d Direction) (cols []OutputColumn) {
	if hasAttrTime {
		cols = append(cols, OUTCOL_TIME)
	}

	if hasAttrIface {
		cols = append(cols, OUTCOL_IFACE)
	}

	for _, attrib := range attributes {
		switch attrib.Name() {
		case "sip":
			cols = append(cols, OUTCOL_SIP)
		case "dip":
			cols = append(cols, OUTCOL_DIP)
		case "proto":
			cols = append(cols, OUTCOL_PROTO)
		case "dport":
			cols = append(cols, OUTCOL_DPORT)
		}
	}

	switch d {
	case DIRECTION_IN:
		cols = append(cols,
			OUTCOL_INPKTS,
			OUTCOL_INPKTSPERCENT,
			OUTCOL_INBYTES,
			OUTCOL_INBYTESPERCENT)
	case DIRECTION_OUT:
		cols = append(cols,
			OUTCOL_OUTPKTS,
			OUTCOL_OUTPKTSPERCENT,
			OUTCOL_OUTBYTES,
			OUTCOL_OUTBYTESPERCENT)
	case DIRECTION_BOTH:
		cols = append(cols,
			OUTCOL_BOTHPKTSRCVD,
			OUTCOL_BOTHPKTSSENT,
			OUTCOL_BOTHPKTSPERCENT,
			OUTCOL_BOTHBYTESRCVD,
			OUTCOL_BOTHBYTESSENT,
			OUTCOL_BOTHBYTESPERCENT)
	case DIRECTION_SUM:
		cols = append(cols,
			OUTCOL_SUMPKTS,
			OUTCOL_SUMPKTSPERCENT,
			OUTCOL_SUMBYTES,
			OUTCOL_SUMBYTESPERCENT)
	}

	return
}

// A formatter provides methods for printing various types/units of values.
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
	case OUTCOL_TIME:
		return format.Time(e.k.Time)
	case OUTCOL_IFACE:
		return format.String(e.k.Iface)

	case OUTCOL_SIP:
		ip := goDB.SipAttribute{}.ExtractStrings(&e.k)[0]
		return format.String(tryLookup(ips2domains, ip))
	case OUTCOL_DIP:
		ip := goDB.DipAttribute{}.ExtractStrings(&e.k)[0]
		return format.String(tryLookup(ips2domains, ip))
	case OUTCOL_DPORT:
		return format.String(goDB.DportAttribute{}.ExtractStrings(&e.k)[0])
	case OUTCOL_PROTO:
		return format.String(goDB.ProtoAttribute{}.ExtractStrings(&e.k)[0])

	case OUTCOL_INBYTES, OUTCOL_BOTHBYTESRCVD:
		return format.Size(e.nBr)
	case OUTCOL_INBYTESPERCENT:
		return format.Float(float64(100*e.nBr) / float64(nz(totals.BytesRcvd)))
	case OUTCOL_INPKTS, OUTCOL_BOTHPKTSRCVD:
		return format.Count(e.nPr)
	case OUTCOL_INPKTSPERCENT:
		return format.Float(float64(100*e.nPr) / float64(nz(totals.PktsRcvd)))
	case OUTCOL_OUTBYTES, OUTCOL_BOTHBYTESSENT:
		return format.Size(e.nBs)
	case OUTCOL_OUTBYTESPERCENT:
		return format.Float(float64(100*e.nBs) / float64(nz(totals.BytesSent)))
	case OUTCOL_OUTPKTS, OUTCOL_BOTHPKTSSENT:
		return format.Count(e.nPs)
	case OUTCOL_OUTPKTSPERCENT:
		return format.Float(float64(100*e.nPs) / float64(nz(totals.PktsSent)))
	case OUTCOL_SUMBYTES:
		return format.Size(e.nBr + e.nBs)
	case OUTCOL_SUMBYTESPERCENT, OUTCOL_BOTHBYTESPERCENT:
		return format.Float(float64(100*(e.nBr+e.nBs)) / float64(nz(totals.BytesRcvd+totals.BytesSent)))
	case OUTCOL_SUMPKTS:
		return format.Count(e.nPr + e.nPs)
	case OUTCOL_SUMPKTSPERCENT, OUTCOL_BOTHPKTSPERCENT:
		return format.Float(float64(100*(e.nPr+e.nPs)) / float64(nz(totals.PktsRcvd+totals.PktsSent)))
	default:
		panic("unknown OutputColumn value")
	}
}

// extractTotal is similar to extract but extracts a total from totals rather
// than an element of an Entry.
func extractTotal(format Formatter, totals Counts, col OutputColumn) string {
	switch col {
	case OUTCOL_INBYTES, OUTCOL_BOTHBYTESRCVD:
		return format.Size(totals.BytesRcvd)
	case OUTCOL_INPKTS, OUTCOL_BOTHPKTSRCVD:
		return format.Count(totals.PktsRcvd)
	case OUTCOL_OUTBYTES, OUTCOL_BOTHBYTESSENT:
		return format.Size(totals.BytesSent)
	case OUTCOL_OUTPKTS, OUTCOL_BOTHPKTSSENT:
		return format.Count(totals.PktsSent)
	case OUTCOL_SUMBYTES:
		return format.Size(totals.BytesRcvd + totals.BytesSent)
	case OUTCOL_SUMPKTS:
		return format.Count(totals.PktsRcvd + totals.PktsSent)
	default:
		panic("unknown or incorrect OutputColumn value")
	}
}

// describe comes up with a nice string for the given SortOrder and Direction.
func describe(o SortOrder, d Direction) string {
	result := "accumulated "
	switch o {
	case SORT_PACKETS:
		result += "packets "
	case SORT_TRAFFIC:
		result += "data volume "
	case SORT_TIME:
		return "first packet time" // TODO(lob): Is this right?
	}

	switch d {
	case DIRECTION_SUM, DIRECTION_BOTH:
		result += "(sent and received)"
	case DIRECTION_IN:
		result += "(received only)"
	case DIRECTION_OUT:
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
	sort SortOrder,
	hasAttrTime, hasAttrIface bool,
	direction Direction,
	attributes []goDB.Attribute,
	ips2domains map[string]string,
	totalInPkts, totalOutPkts, totalInBytes, totalOutBytes uint64,
	ifaces string,
) basePrinter {
	result := basePrinter{
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

type CSVFormatter struct{}

func (_ CSVFormatter) Size(s uint64) string {
	return fmt.Sprint(s)
}

func (_ CSVFormatter) Duration(d time.Duration) string {
	return fmt.Sprint(d)
}

func (_ CSVFormatter) Count(c uint64) string {
	return fmt.Sprint(c)
}

func (_ CSVFormatter) Float(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

func (_ CSVFormatter) Time(epoch int64) string {
	return fmt.Sprint(epoch)
}

func (_ CSVFormatter) String(s string) string {
	return s
}

type CSVTablePrinter struct {
	basePrinter
	writer *csv.Writer
	fields []string
}

func NewCSVTablePrinter(b basePrinter) *CSVTablePrinter {
	c := CSVTablePrinter{
		b,
		csv.NewWriter(output),
		make([]string, 0, len(b.cols)),
	}

	headers := [COUNT_OUTCOL]string{
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

func (c *CSVTablePrinter) AddRow(entry Entry) {
	c.fields = c.fields[:0]
	for _, col := range c.cols {
		c.fields = append(c.fields, extract(CSVFormatter{}, c.ips2domains, c.totals, entry, col))
	}
	c.writer.Write(c.fields)
}

func (c *CSVTablePrinter) Footer(conditional string, spanFirst, spanLast time.Time, queryDuration, resolveDuration time.Duration) {
	var summaryEntries [COUNT_OUTCOL]string
	summaryEntries[OUTCOL_INPKTS] = "Overall packets"
	summaryEntries[OUTCOL_INBYTES] = "Overall data volume (bytes)"
	summaryEntries[OUTCOL_OUTPKTS] = "Overall packets"
	summaryEntries[OUTCOL_OUTBYTES] = "Overall data volume (bytes)"
	summaryEntries[OUTCOL_SUMPKTS] = "Overall packets"
	summaryEntries[OUTCOL_SUMBYTES] = "Overall data volume (bytes)"
	summaryEntries[OUTCOL_BOTHPKTSRCVD] = "Received packets"
	summaryEntries[OUTCOL_BOTHPKTSSENT] = "Sent packets"
	summaryEntries[OUTCOL_BOTHBYTESRCVD] = "Received data volume (bytes)"
	summaryEntries[OUTCOL_BOTHBYTESSENT] = "Sent data volume (bytes)"
	for _, col := range c.cols {
		if summaryEntries[col] != "" {
			c.writer.Write([]string{summaryEntries[col], extractTotal(CSVFormatter{}, c.totals, col)})
		}
	}
	c.writer.Write([]string{"Sorting and flow direction", describe(c.sort, c.direction)})
	c.writer.Write([]string{"Interface", c.ifaces})
}

func (c *CSVTablePrinter) Print() error {
	c.writer.Flush()
	return nil
}

type JSONFormatter struct{}

func (_ JSONFormatter) Size(s uint64) string {
	result, _ := json.Marshal(s)
	return string(result)
}

func (_ JSONFormatter) Duration(d time.Duration) string {
	result, _ := json.Marshal(d)
	return string(result)

}

func (_ JSONFormatter) Count(c uint64) string {
	result, _ := json.Marshal(c)
	return string(result)

}

func (_ JSONFormatter) Float(f float64) string {
	result, _ := json.Marshal(f)
	return string(result)

}

func (_ JSONFormatter) Time(epoch int64) string {
	// convert to string first for legacy reasons
	result, _ := json.Marshal(fmt.Sprint(epoch))
	return string(result)
}

func (_ JSONFormatter) String(s string) string {
	result, _ := json.Marshal(s)
	return string(result)
}

var jsonKeys = [COUNT_OUTCOL]string{
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

type JSONTablePrinter struct {
	basePrinter
	rows      []map[string]*json.RawMessage
	data      map[string]interface{}
	queryType string
}

func NewJSONTablePrinter(b basePrinter, queryType string) *JSONTablePrinter {
	j := JSONTablePrinter{
		b,
		nil,
		make(map[string]interface{}),
		queryType,
	}

	return &j
}

func (j *JSONTablePrinter) AddRow(entry Entry) {
	row := make(map[string]*json.RawMessage)
	for _, col := range j.cols {
		val := json.RawMessage(extract(JSONFormatter{}, j.ips2domains, j.totals, entry, col))
		row[jsonKeys[col]] = &val
	}
	j.rows = append(j.rows, row)
}

func (j *JSONTablePrinter) Footer(conditional string, spanFirst, spanLast time.Time, queryDuration, resolveDuration time.Duration) {
	j.data["status"] = "ok"
	j.data["ext_ips"] = externalIPs()

	summary := map[string]interface{}{
		"interface": j.ifaces,
	}
	var summaryEntries [COUNT_OUTCOL]string
	summaryEntries[OUTCOL_INPKTS] = "total_packets"
	summaryEntries[OUTCOL_INBYTES] = "total_bytes"
	summaryEntries[OUTCOL_OUTPKTS] = "total_packets"
	summaryEntries[OUTCOL_OUTBYTES] = "total_bytes"
	summaryEntries[OUTCOL_SUMPKTS] = "total_packets"
	summaryEntries[OUTCOL_SUMBYTES] = "total_bytes"
	summaryEntries[OUTCOL_BOTHPKTSRCVD] = "total_packets_rcvd"
	summaryEntries[OUTCOL_BOTHPKTSSENT] = "total_packets_sent"
	summaryEntries[OUTCOL_BOTHBYTESRCVD] = "total_bytes_rcvd"
	summaryEntries[OUTCOL_BOTHBYTESSENT] = "total_bytes_sent"
	for _, col := range j.cols {
		if summaryEntries[col] != "" {
			val := json.RawMessage(extractTotal(JSONFormatter{}, j.totals, col))
			summary[summaryEntries[col]] = &val
		}
	}

	j.data["summary"] = summary
}

func (j *JSONTablePrinter) Print() error {
	j.data[j.queryType] = j.rows
	return json.NewEncoder(output).Encode(j.data)
}

type TextFormatter struct{}

func (_ TextFormatter) Size(size uint64) string {
	count := 0
	var sizeF float64 = float64(size)

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

func (_ TextFormatter) Duration(d time.Duration) string {
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

func (_ TextFormatter) Count(val uint64) string {
	count := 0
	var valF float64 = float64(val)

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

func (_ TextFormatter) Float(f float64) string {
	if f == 0 {
		return fmt.Sprintf("%.2f", f)
	}
	return fmt.Sprintf("%.2f", f)
}

func (_ TextFormatter) Time(epoch int64) string {
	return time.Unix(epoch, 0).Format("06-01-02 15:04:05")
}

func (_ TextFormatter) String(s string) string {
	return s
}

type TextTablePrinter struct {
	basePrinter
	writer         *tabwriter.Writer
	footwriter     *tabwriter.Writer
	numFlows       int
	resolveTimeout time.Duration
	numPrinted     int
}

func NewTextTablePrinter(b basePrinter, numFlows int, resolveTimeout time.Duration) *TextTablePrinter {
	var t = &TextTablePrinter{
		b,
		tabwriter.NewWriter(output, 0, 1, 2, ' ', tabwriter.AlignRight),
		tabwriter.NewWriter(output, 0, 4, 1, ' ', 0),
		numFlows,
		resolveTimeout,
		0,
	}

	var header1 [COUNT_OUTCOL]string
	header1[OUTCOL_INPKTS] = "packets"
	header1[OUTCOL_INBYTES] = "bytes"
	header1[OUTCOL_OUTPKTS] = "packets"
	header1[OUTCOL_OUTBYTES] = "bytes"
	header1[OUTCOL_SUMPKTS] = "packets"
	header1[OUTCOL_SUMBYTES] = "bytes"
	header1[OUTCOL_BOTHPKTSRCVD] = "packets"
	header1[OUTCOL_BOTHPKTSSENT] = "packets"
	header1[OUTCOL_BOTHBYTESRCVD] = "bytes"
	header1[OUTCOL_BOTHBYTESSENT] = "bytes"

	var header2 = [COUNT_OUTCOL]string{
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

func (t *TextTablePrinter) AddRow(entry Entry) {
	for _, col := range t.cols {
		fmt.Fprint(t.writer, extract(TextFormatter{}, t.ips2domains, t.totals, entry, col))
		fmt.Fprint(t.writer, "\t")
	}
	fmt.Fprintln(t.writer)
	t.numPrinted++
}

func (t *TextTablePrinter) Footer(conditional string, spanFirst, spanLast time.Time, queryDuration, resolveDuration time.Duration) {
	var isTotal [COUNT_OUTCOL]bool
	isTotal[OUTCOL_INPKTS] = true
	isTotal[OUTCOL_INBYTES] = true
	isTotal[OUTCOL_OUTPKTS] = true
	isTotal[OUTCOL_OUTBYTES] = true
	isTotal[OUTCOL_SUMPKTS] = true
	isTotal[OUTCOL_SUMBYTES] = true
	isTotal[OUTCOL_BOTHPKTSRCVD] = true
	isTotal[OUTCOL_BOTHPKTSSENT] = true
	isTotal[OUTCOL_BOTHBYTESRCVD] = true
	isTotal[OUTCOL_BOTHBYTESSENT] = true

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

	if t.direction == DIRECTION_BOTH {
		for range t.cols[1:] {
			fmt.Fprint(t.writer, "\t")
		}
		fmt.Fprintln(t.writer)

		fmt.Fprint(t.writer, "Totals:\t")
		for _, col := range t.cols[1:] {
			if col == OUTCOL_BOTHPKTSSENT {
				fmt.Fprint(t.writer, TextFormatter{}.Count(t.totals.PktsRcvd+t.totals.PktsSent))
			}
			if col == OUTCOL_BOTHBYTESSENT {
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

func (t *TextTablePrinter) Print() error {
	fmt.Fprintln(output) // newline between prompt and results
	t.writer.Flush()
	fmt.Fprintln(output)
	t.footwriter.Flush()
	fmt.Fprintln(output)

	return nil
}

// The term 'key' has two different meanings in the InfluxDB documentation.
// Here we mean key as in "the key field of a protocol line '[key] [fields] [timestamp]'".
const INFLUXDB_KEY = "goprobe_flows"

// The term 'key' has two different meanings in the InfluxDB documentation.
// Here we mean key as in "key-value metric". Key-value metrics are needed for
// specifying tags and fields.
var influxDBKeys = [COUNT_OUTCOL]string{
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

// See https://docs.influxdata.com/influxdb/v0.10/write_protocols/line/
// for details on the InfluxDB line protocol.
// Important detail: InfluxDB  wants integral values to have the suffix 'i'.
// Floats have no suffix.
type InfluxDBFormatter struct{}

func (_ InfluxDBFormatter) Size(s uint64) string {
	return fmt.Sprintf("%di", s)
}

func (_ InfluxDBFormatter) Duration(d time.Duration) string {
	return fmt.Sprintf("%di", d.Nanoseconds())
}

func (_ InfluxDBFormatter) Count(c uint64) string {
	return fmt.Sprintf("%di", c)
}

func (_ InfluxDBFormatter) Float(f float64) string {
	return fmt.Sprint(f)
}

func (_ InfluxDBFormatter) Time(epoch int64) string {
	// InfluxDB prefers nanosecond epoch timestamps
	return fmt.Sprint(epoch * int64(time.Second))
}

// Limitation: Since we only use strings in tags, we only escape strings for the
// tag format, not for the field format.
func (_ InfluxDBFormatter) String(s string) string {
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

type InfluxDBTablePrinter struct {
	basePrinter
	tagCols, fieldCols []OutputColumn
}

// Implements sort.Interface for []OutputColumn
type ByInfluxDBKey []OutputColumn

func (xs ByInfluxDBKey) Len() int {
	return len(xs)
}

func (xs ByInfluxDBKey) Less(i, j int) bool {
	return bytes.Compare([]byte(influxDBKeys[xs[i]]), []byte(influxDBKeys[xs[j]])) < 0
}

func (xs ByInfluxDBKey) Swap(i, j int) {
	xs[i], xs[j] = xs[j], xs[i]
}

func NewInfluxDBTablePrinter(b basePrinter) *InfluxDBTablePrinter {
	var isTagCol, isFieldCol [COUNT_OUTCOL]bool
	// OUTCOL_TIME is no tag and no field
	isTagCol[OUTCOL_IFACE] = true
	isFieldCol[OUTCOL_SIP] = true
	isFieldCol[OUTCOL_DIP] = true
	isFieldCol[OUTCOL_DPORT] = true
	isTagCol[OUTCOL_PROTO] = true
	isFieldCol[OUTCOL_INPKTS] = true
	// ignore OUTCOL_INPKTSPERCENT
	isFieldCol[OUTCOL_INBYTES] = true
	// ignore OUTCOL_INBYTESPERCENT
	isFieldCol[OUTCOL_OUTPKTS] = true
	// ignore OUTCOL_OUTPKTSPERCENT
	isFieldCol[OUTCOL_OUTBYTES] = true
	// ignore OUTCOL_OUTBYTESPERCENT
	isFieldCol[OUTCOL_SUMPKTS] = true
	// ignore OUTCOL_SUMPKTSPERCENT
	isFieldCol[OUTCOL_SUMBYTES] = true
	// ignore OUTCOL_SUMBYTESPERCENT
	isFieldCol[OUTCOL_BOTHPKTSRCVD] = true
	isFieldCol[OUTCOL_BOTHPKTSSENT] = true
	// ignore OUTCOL_BOTHPKTSPERCENT
	isFieldCol[OUTCOL_BOTHBYTESRCVD] = true
	isFieldCol[OUTCOL_BOTHBYTESSENT] = true
	// ignore OUTCOL_BOTHBYTESPERCENT

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

func (i *InfluxDBTablePrinter) AddRow(entry Entry) {
	// Key + tags
	fmt.Fprint(output, INFLUXDB_KEY)
	for _, col := range i.tagCols {
		fmt.Fprint(output, ",")
		fmt.Fprint(output, influxDBKeys[col])
		fmt.Fprint(output, "=")
		fmt.Fprint(output, extract(TextFormatter{}, i.ips2domains, i.totals, entry, col))
	}

	fmt.Fprint(output, " ")

	// Fields
	fmt.Fprint(output, influxDBKeys[i.fieldCols[0]])
	fmt.Fprint(output, "=")
	fmt.Fprint(output, extract(InfluxDBFormatter{}, i.ips2domains, i.totals, entry, i.fieldCols[0]))
	for _, col := range i.fieldCols[1:] {
		fmt.Fprint(output, ",")
		fmt.Fprint(output, influxDBKeys[col])
		fmt.Fprint(output, "=")
		fmt.Fprint(output, extract(InfluxDBFormatter{}, i.ips2domains, i.totals, entry, col))
	}

	// Time
	if i.hasAttrTime {
		fmt.Fprint(output, " ")
		fmt.Fprint(output, extract(InfluxDBFormatter{}, i.ips2domains, i.totals, entry, OUTCOL_TIME))
	}

	fmt.Fprintln(output)
}

func (_ *InfluxDBTablePrinter) Footer(conditional string, spanFirst, spanLast time.Time, queryDuration, resolveDuration time.Duration) {
}

func (_ *InfluxDBTablePrinter) Print() error {
	return nil
}

// NewTablePrinter provides a convenient interface for instantiating the various
// TablePrinters. You could call it a factory method.
func NewTablePrinter(
	config Config, // TODO(lob): I am not a fan of passing around the huge Config
	sort SortOrder,
	d Direction,
	hasAttrTime, hasAttrIface bool,
	attributes []goDB.Attribute,
	ips2domains map[string]string,
	sums Counts,
	numFlows int,
) (TablePrinter, error) {

	b := makeBasePrinter(
		sort,
		hasAttrTime, hasAttrIface,
		d,
		attributes,
		ips2domains,
		sums.PktsRcvd, sums.PktsSent, sums.BytesRcvd, sums.BytesSent,
		config.Ifaces)

	switch config.Format {
	case "txt":
		return NewTextTablePrinter(b, numFlows, config.ResolveTimeout), nil
	case "json":
		return NewJSONTablePrinter(b, config.QueryType), nil
	case "csv":
		return NewCSVTablePrinter(b), nil
	case "influxdb":
		return NewInfluxDBTablePrinter(b), nil
	default:
		return nil, fmt.Errorf("Unknown output format %s", config.Format)
	}
}
