package query

import (
	"fmt"
	"io"
	"time"

	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/pkg/types"
)

// Statement bundles all relevant options for running a query and displaying its result
type Statement struct {
	// Ifaces holds the list of all interfaces that should be queried
	Ifaces []string `json:"ifaces"`

	LabelSelector types.LabelSelector `json:"label_selector,omitempty"`

	// needed for feedback to user
	QueryType string `json:"query_type"`

	attributes []types.Attribute `json:"-"`
	Condition  string            `json:"condition,omitempty"`

	// which direction is added
	Direction types.Direction `json:"direction"`

	// time selection
	First       int64         `json:"from"`
	Last        int64         `json:"to"`
	TimeBinSize time.Duration `json:"time_bin_size"`

	// formatting
	Format        string            `json:"format"`
	NumResults    uint64            `json:"limit"`
	SortBy        results.SortOrder `json:"sort_by"`
	SortAscending bool              `json:"sort_ascending,omitempty"`
	Output        io.Writer         `json:"-"`

	// parameters for external calls
	Caller string `json:"caller,omitempty"` // who called the query

	// resolution parameters (probably part of table printer)
	DNSResolution DNSResolution `json:"dns_resolution,omitempty"`

	// file system
	MaxMemPct int  `json:"max_mem_pct,omitempty"`
	LowMem    bool `json:"low_mem,omitempty"`

	// query keepalive
	KeepAliveDuration time.Duration `json:"keepalive,omitempty"`

	// request live flow data (in addition to DB)
	Live bool `json:"live,omitempty"`
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
	str += fmt.Sprintf(", limit: %d, from: %s, to: %s",
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

func (s *Statement) Pretty() string {
	ifaces := "any"
	if len(s.Ifaces) > 0 {
		ifaces = fmt.Sprintf("%v", s.Ifaces)
	}
	str := fmt.Sprintf(`
    query: %s
   ifaces: %s
`, s.QueryType, ifaces)

	if s.Condition != "" {
		str += fmt.Sprintf(`
condition: %s
		`, s.Condition)
	}
	tFrom, tTo := time.Unix(s.First, 0), time.Unix(s.Last, 0)
	str += fmt.Sprintf(`
     from: %s
       to: %s

    limit: %d
`,
		tFrom.Format(time.ANSIC),
		tTo.Format(time.ANSIC),
		s.NumResults,
	)
	if s.DNSResolution.Enabled {
		str += fmt.Sprintf(`
      dns: %t
`, s.DNSResolution.Enabled)
	}
	return str
}
