/////////////////////////////////////////////////////////////////////////////////
//
// Query.go
//
// Defines a Query struct that contains the attributes queried and a conditional
// determining which values are considered, as well as meta-information to make
// query evaluation easier.
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

type columnIndex int

// Indizes for all column types
const (
	// First the attribute columns...
	SipColIdx, _ columnIndex = iota, iota
	DipColIdx, _
	ProtoColIdx, _
	DportColIdx, _

	// ... and then the columns we aggregate
	BytesRcvdColIdx, ColIdxAttributeCount
	BytesSentColIdx, _
	PacketsRcvdColIdx, _
	PacketsSentColIdx, _
	ColIdxCount, _
)

// Sizeof (entry) for all column types
const (
	SipSizeof   int = 16
	DipSizeof   int = 16
	ProtoSizeof int = 1
	DportSizeof int = 2
)

// IsCounterCol returns if a column is a counter (and hence does
// not use fixed-width encoding)
func (c columnIndex) IsCounterCol() bool {
	return c >= ColIdxAttributeCount && c <= PacketsSentColIdx
}

var columnSizeofs = [ColIdxCount]int{
	SipSizeof, DipSizeof, ProtoSizeof, DportSizeof,
}

var columnFileNames = [ColIdxCount]string{
	"sip", "dip", "proto", "dport",
	"bytes_rcvd", "bytes_sent", "pkts_rcvd", "pkts_sent"}

// Query stores all relevant parameters for data selection
type Query struct {
	// list of attributes that will be compared, e.g. "dip" "sip"
	// in a "talk_conv" query
	Attributes  []Attribute
	Conditional Node

	hasAttrTime, hasAttrIface bool

	// Each of the following slices represents a set in the sense that each column index can occur at most once in each slice.
	// They are populated during the call to NewQuery

	// Set of indizes of all attributes used in the query, except for the special "time" attribute.
	// Example: For query type talk_conv, queryAttributeIndizes would contain SipColIdx and DipColIdx
	queryAttributeIndizes []columnIndex
	// Set of indizes of all attributes used in the conditional.
	// Example: For the conditional "dport = 80 & dnet = 0.0.0.0/0" conditionalAttributeIndizes
	// would contain DipColIdx and DportColIdx
	conditionalAttributeIndizes []columnIndex
	// Set containing the union of queryAttributeIndizes, conditionalAttributeIndizes, and
	// {BytesSentColIdx, PacketsRcvdColIdx, PacketsSentColIdx, ColIdxCount}.
	// The latter four elements are needed for every query since they contain the variables we aggregate.
	columnIndizes []columnIndex
}

// Computes a columnIndex from a column name. In principle we could merge
// this function with conditionalAttributeNameToColumnIndex; however, then
// we wouldn't "fail early" if an snet or dnet entry somehow made it into
// the condition attributes.
func queryAttributeNameToColumnIndex(name string) (colIdx columnIndex) {
	colIdx, ok := map[string]columnIndex{
		"sip":   SipColIdx,
		"dip":   DipColIdx,
		"proto": ProtoColIdx,
		"dport": DportColIdx}[name]
	if !ok {
		panic("Unknown query attribute " + name)
	}
	return
}

// Computes a columnIndex from a column name. Different from queryAttributeNameToColumnIndex
// because snet and dnet are only allowed in conditionals.
func conditionalAttributeNameToColumnIndex(name string) (colIdx columnIndex) {
	colIdx, ok := map[string]columnIndex{
		"sip":   SipColIdx,
		"snet":  SipColIdx,
		"dip":   DipColIdx,
		"dnet":  DipColIdx,
		"proto": ProtoColIdx,
		"dport": DportColIdx}[name]
	if !ok {
		panic("Unknown conditional attribute " + name)
	}
	return
}

// NewQuery creates a new Query object based on the parsed command line parameters
func NewQuery(attributes []Attribute, conditional Node, hasAttrTime, hasAttrIface bool) *Query {
	q := &Query{
		Attributes:   attributes,
		Conditional:  conditional,
		hasAttrTime:  hasAttrTime,
		hasAttrIface: hasAttrIface,
	}

	// Compute index sets
	var isAttributeIndex [ColIdxAttributeCount]bool // temporary variable for computing set union

	for _, attrib := range q.Attributes {
		colIdx := queryAttributeNameToColumnIndex(attrib.Name())
		q.queryAttributeIndizes = append(q.queryAttributeIndizes, colIdx)
		isAttributeIndex[colIdx] = true
	}

	if q.Conditional != nil {
		for attribName := range q.Conditional.attributes() {
			colIdx := conditionalAttributeNameToColumnIndex(attribName)
			q.conditionalAttributeIndizes = append(q.conditionalAttributeIndizes, colIdx)
			isAttributeIndex[colIdx] = true
		}
	}
	for colIdx := columnIndex(0); colIdx < ColIdxAttributeCount; colIdx++ {
		if isAttributeIndex[colIdx] {
			q.columnIndizes = append(q.columnIndizes, colIdx)
		}
	}
	q.columnIndizes = append(q.columnIndizes,
		BytesRcvdColIdx, BytesSentColIdx, PacketsRcvdColIdx, PacketsSentColIdx)

	return q
}

// AttributesToString is a convenience method for translating the query attributes
// into a human-readable name
func (q *Query) AttributesToString() []string {
	s := make([]string, len(q.Attributes))
	for i, a := range q.Attributes {
		s[i] = a.Name()
	}
	return s
}
