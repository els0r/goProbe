package query

import (
	"fmt"
	"io"
	"time"

	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
)

// Statement bundles all relevant options for running a query and displaying its result
type Statement struct {
	// Ifaces holds hte list of all interfaces that should be queried
	Ifaces []string `json:"ifaces"`

	HasAttrIface bool `json:"-"`
	HasAttrTime  bool `json:"-"`

	// needed for feedback to user
	QueryType string `json:"query_type"`

	Attributes []types.Attribute `json:"attributes"`
	Condition  string            `json:"condition,omitempty"`

	// which direction is added
	Direction types.Direction `json:"direction"`

	// time selection
	First int64 `json:"from"`
	Last  int64 `json:"to"`

	// formatting
	Format        string            `json:"format"`
	NumResults    int               `json:"limit"`
	SortBy        results.SortOrder `json:"sort_by"`
	SortAscending bool              `json:"sort_ascending,omitempty"`
	Output        io.Writer         `json:"-"`

	// parameters for external calls
	Caller string `json:"caller,omitempty"` // who called the query

	// resolution parameters (probably part of table printer)
	DNSResolution struct {
		Enabled bool          `json:"enabled,omitempty"`
		Timeout time.Duration `json:"dns_timeout,omitempty"`
		MaxRows int           `json:"max_rows,omitempty"`
	} `json:"dns_resolution,omitempty"`

	// file system
	DBPath    string `json:"db"`
	MaxMemPct int    `json:"-"`
	LowMem    bool   `json:"low_mem,omitempty"`

	// error during execution
	Err error `json:"error,omitempty"`
}

// String prints the executable statement in human-readable form
func (s *Statement) String() string {
	str := fmt.Sprintf("{type: %s, ifaces: %s",
		s.QueryType,
		s.Ifaces,
	)
	if s.Condition != "" {
		str += fmt.Sprintf(", condition: %s", s.Condition)
	}
	tFrom, tTo := time.Unix(s.First, 0), time.Unix(s.Last, 0)
	str += fmt.Sprintf(", db: %s, limit: %d, from: %s, to: %s",
		s.DBPath,
		s.NumResults,
		tFrom.Format(time.ANSIC),
		tTo.Format(time.ANSIC),
	)
	if s.DNSResolution.Enabled {
		str += fmt.Sprintf(", dns-resolution: %t", s.DNSResolution.Enabled)
	}
	str += "}"
	return str
}
