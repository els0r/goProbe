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

import (
	"time"

	"github.com/els0r/goProbe/pkg/goDB/conditions/node"
	"github.com/els0r/goProbe/pkg/types"
)

// Query stores all relevant parameters for data selection
type Query struct {
	// list of attributes that will be compared, e.g. "dip" "sip"
	// in a "talk_conv" query
	Attributes  []types.Attribute
	Conditional node.Node

	// Explicity attribute flags that allow granular processing logic
	// without having to rely on array loops
	hasAttrTime, hasAttrIface                          bool
	hasAttrSIP, hasAttrDIP, hasAttrDport, hasAttrProto bool
	hasCondSIP, hasCondDIP, hasCondDport, hasCondProto bool
	ipVersion                                          types.IPVersion

	// metadataOnly will determine if all relevant information to answer the query can be
	// derived solely from metadata inside GPDir
	metadataOnly bool

	// Each of the following slices represents a set in the sense that each column index can occur at most once in each slice.
	// They are populated during the call to NewQuery

	// Set of indices of all attributes used in the query, except for the special "time" attribute.
	// Example: For query type talk_conv, queryAttributeIndices would contain SipColIdx and DipColIdx
	queryAttributeIndices []types.ColumnIndex
	// Set of indices of all attributes used in the conditional.
	// Example: For the conditional "dport = 80 & dnet = 0.0.0.0/0" conditionalAttributeIndices
	// would contain DipColIdx and DportColIdx
	conditionalAttributeIndices []types.ColumnIndex
	// Set containing the union of queryAttributeIndices, conditionalAttributeIndices, and
	// {BytesSentColIdx, PacketsRcvdColIdx, PacketsSentColIdx, ColIdxCount}.
	// The latter four elements are needed for every query since they contain the variables we aggregate.
	columnIndices []types.ColumnIndex

	// Enables memory-saving mode
	lowMem bool

	// Query keep-alive tracking
	lastKeepalive     time.Time
	keepaliveInterval time.Duration
	keepaliveFn       func()
}

// Computes a columnIndex from a column name. In principle we could merge
// this function with conditionalAttributeNameToColumnIndex; however, then
// we wouldn't "fail early" if an snet or dnet entry somehow made it into
// the condition attributes.
func queryAttributeNameToColumnIndex(name string) (colIdx types.ColumnIndex) {
	colIdx, ok := map[string]types.ColumnIndex{
		types.SIPName:   types.SIPColIdx,
		types.DIPName:   types.DIPColIdx,
		types.ProtoName: types.ProtoColIdx,
		types.DportName: types.DportColIdx}[name]
	if !ok {
		panic("Unknown query attribute " + name)
	}
	return
}

// Computes a columnIndex from a column name. Different from queryAttributeNameToColumnIndex
// because snet and dnet are only allowed in conditionals.
func conditionalAttributeNameToColumnIndex(name string) (colIdx types.ColumnIndex) {
	colIdx, ok := map[string]types.ColumnIndex{
		types.SIPName:   types.SIPColIdx,
		"snet":          types.SIPColIdx,
		types.DIPName:   types.DIPColIdx,
		"dnet":          types.DIPColIdx,
		types.ProtoName: types.ProtoColIdx,
		types.DportName: types.DportColIdx}[name]
	if !ok {
		panic("Unknown conditional attribute " + name)
	}
	return
}

var queryAttributeColumnFlagSetters = [types.ColIdxAttributeCount]func(q *Query){
	func(q *Query) { q.hasAttrSIP = true },
	func(q *Query) { q.hasAttrDIP = true },
	func(q *Query) { q.hasAttrProto = true },
	func(q *Query) { q.hasAttrDport = true },
}

var queryConditionalColumnFlagSetters = [types.ColIdxAttributeCount]func(q *Query){
	func(q *Query) { q.hasCondSIP = true },
	func(q *Query) { q.hasCondDIP = true },
	func(q *Query) { q.hasCondProto = true },
	func(q *Query) { q.hasCondDport = true },
}

// NewMetadataQuery creates a metadata-only query
func NewMetadataQuery() *Query {
	q := NewQuery([]types.Attribute{}, nil, types.LabelSelector{Iface: true})
	q.metadataOnly = true

	return q
}

// NewQuery creates a new Query object based on the parsed command line parameters
func NewQuery(attributes []types.Attribute, conditional node.Node, selector types.LabelSelector) *Query {
	q := &Query{
		Attributes:   attributes,
		Conditional:  conditional,
		hasAttrTime:  selector.Timestamp,
		hasAttrIface: selector.Iface,
	}

	// Compute index sets
	var isAttributeIndex [types.ColIdxAttributeCount]bool // temporary variable for computing set union

	for _, attrib := range q.Attributes {
		colIdx := queryAttributeNameToColumnIndex(attrib.Name())
		q.queryAttributeIndices = append(q.queryAttributeIndices, colIdx)
		isAttributeIndex[colIdx] = true
		queryAttributeColumnFlagSetters[colIdx](q)
	}

	if q.Conditional != nil {
		for attribName, ipVersion := range q.Conditional.Attributes() {
			colIdx := conditionalAttributeNameToColumnIndex(attribName)
			q.conditionalAttributeIndices = append(q.conditionalAttributeIndices, colIdx)
			isAttributeIndex[colIdx] = true
			queryConditionalColumnFlagSetters[colIdx](q)
			q.ipVersion = q.ipVersion.Merge(ipVersion)
		}
	}
	for colIdx := types.ColumnIndex(0); colIdx < types.ColIdxAttributeCount; colIdx++ {
		if isAttributeIndex[colIdx] {
			q.columnIndices = append(q.columnIndices, colIdx)
		}
	}
	q.columnIndices = append(q.columnIndices,
		types.BytesRcvdColIdx, types.BytesSentColIdx, types.PacketsRcvdColIdx, types.PacketsSentColIdx)

	return q
}

// LowMem enables memory-saving mode
func (q *Query) LowMem(enable bool) *Query {
	q.lowMem = enable
	return q
}

// Keepalive enables sending keepalives at a given frequency
func (q *Query) Keepalive(fn func(), interval time.Duration) *Query {
	q.keepaliveFn = fn
	q.keepaliveInterval = interval
	return q
}

// IsLowMem returns if the query was run in low-memory mode
func (q *Query) IsLowMem() bool {
	return q.lowMem
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

// UpdateKeepalive emits a specific log line if enabled and a minimum time period has elapsed
func (q *Query) UpdateKeepalive() {
	if q.keepaliveInterval > 0 && q.keepaliveFn != nil {
		// assess time since last keepalive emission and act accordingly
		if time.Since(q.lastKeepalive) > q.keepaliveInterval {
			q.lastKeepalive = time.Now()
			q.keepaliveFn()
		}
	}
}
