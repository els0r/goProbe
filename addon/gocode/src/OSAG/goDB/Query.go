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
	SIP_COLIDX, _ columnIndex = iota, iota
	DIP_COLIDX, _
	PROTO_COLIDX, _
	DPORT_COLIDX, _
	// ... and then the columns we aggregate
	BYTESRCVD_COLIDX, COLIDX_ATTRIBUTE_COUNT
	BYTESSENT_COLIDX, _
	PKTSRCVD_COLIDX, _
	PKTSSENT_COLIDX, _
	COLIDX_COUNT, _
)

// Sizeof (entry) for all column types
const (
	SIP_SIZEOF       int = 16
	DIP_SIZEOF       int = 16
	PROTO_SIZEOF     int = 1
	DPORT_SIZEOF     int = 2
	BYTESRCVD_SIZEOF int = 8
	BYTESSENT_SIZEOF int = 8
	PKTSRCVD_SIZEOF  int = 8
	PKTSSENT_SIZEOF  int = 8
)

var columnSizeofs = [COLIDX_COUNT]int{
	SIP_SIZEOF, DIP_SIZEOF, PROTO_SIZEOF, DPORT_SIZEOF,
	BYTESRCVD_SIZEOF, BYTESSENT_SIZEOF, PKTSRCVD_SIZEOF, PKTSSENT_SIZEOF}

var columnFileNames = [COLIDX_COUNT]string{
	"sip", "dip", "proto", "dport",
	"bytes_rcvd", "bytes_sent", "pkts_rcvd", "pkts_sent"}

type Query struct {
	// list of attributes that will be compared, e.g. "dip" "sip"
	// in a "talk_conv" query
	Attributes  []Attribute
	Conditional Node

	hasAttrTime, hasAttrIface bool

	// Each of the following slices represents a set in the sense that each column index can occur at most once in each slice.
	// They are populated during the call to NewQuery

	// Set of indizes of all attributes used in the query, except for the special "time" attribute.
	// Example: For query type talk_conv, queryAttributeIndizes would contain SIP_COLIDX and DIP_COLIDX
	queryAttributeIndizes []columnIndex
	// Set of indizes of all attributes used in the conditional.
	// Example: For the conditional "dport = 80 & dnet = 0.0.0.0/0" conditionalAttributeIndizes
	// would contain DIP_COLIDX and DPORT_COLIDX
	conditionalAttributeIndizes []columnIndex
	// Set containing the union of queryAttributeIndizes, conditionalAttributeIndizes, and
	// {BYTESSENT_COLIDX, PKTSRCVD_COLIDX, PKTSSENT_COLIDX, COLIDX_COUNT}.
	// The latter four elements are needed for every query since they contain the variables we aggregate.
	columnIndizes []columnIndex
}

// Computes a columnIndex from a column name. In principle we could merge
// this function with conditionalAttributeNameToColumnIndex; however, then
// we wouldn't "fail early" if an snet or dnet entry somehow made it into
// the condition attributes.
func queryAttributeNameToColumnIndex(name string) (colIdx columnIndex) {
	colIdx, ok := map[string]columnIndex{
		"sip":   SIP_COLIDX,
		"dip":   DIP_COLIDX,
		"proto": PROTO_COLIDX,
		"dport": DPORT_COLIDX}[name]
	if !ok {
		panic("Unknown query attribute " + name)
	}
	return
}

// Computes a columnIndex from a column name. Different from queryAttributeNameToColumnIndex
// because snet and dnet are only allowed in conditionals.
func conditionalAttributeNameToColumnIndex(name string) (colIdx columnIndex) {
	colIdx, ok := map[string]columnIndex{
		"sip":   SIP_COLIDX,
		"snet":  SIP_COLIDX,
		"dip":   DIP_COLIDX,
		"dnet":  DIP_COLIDX,
		"proto": PROTO_COLIDX,
		"dport": DPORT_COLIDX}[name]
	if !ok {
		panic("Unknown conditional attribute " + name)
	}
	return
}

func NewQuery(attributes []Attribute, conditional Node, hasAttrTime, hasAttrIface bool) *Query {
	q := &Query{
		Attributes:   attributes,
		Conditional:  conditional,
		hasAttrTime:  hasAttrTime,
		hasAttrIface: hasAttrIface,
	}

	// Compute index sets
	var isAttributeIndex [COLIDX_ATTRIBUTE_COUNT]bool // temporary variable for computing set union

	for _, attrib := range q.Attributes {
		colIdx := queryAttributeNameToColumnIndex(attrib.Name())
		q.queryAttributeIndizes = append(q.queryAttributeIndizes, colIdx)
		isAttributeIndex[colIdx] = true
	}

	if q.Conditional != nil {
		for attribName, _ := range q.Conditional.attributes() {
			colIdx := conditionalAttributeNameToColumnIndex(attribName)
			q.conditionalAttributeIndizes = append(q.conditionalAttributeIndizes, colIdx)
			isAttributeIndex[colIdx] = true
		}
	}
	for colIdx := columnIndex(0); colIdx < COLIDX_ATTRIBUTE_COUNT; colIdx++ {
		if isAttributeIndex[colIdx] {
			q.columnIndizes = append(q.columnIndizes, colIdx)
		}
	}
	q.columnIndizes = append(q.columnIndizes,
		BYTESRCVD_COLIDX, BYTESSENT_COLIDX, PKTSRCVD_COLIDX, PKTSSENT_COLIDX)

	return q
}
