package results

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/els0r/goProbe/v4/pkg/formatting"
	"github.com/els0r/goProbe/v4/pkg/types"
)

// FooterTabwriter is a specific tabwriter for the results footer
type FooterTabwriter struct {
	*tabwriter.Writer
}

// NewFooterTabwriter returns a new FooterTabwriter
func NewFooterTabwriter(w io.Writer) *FooterTabwriter {
	return &FooterTabwriter{
		tabwriter.NewWriter(w, 0, 4, 1, ' ', 0),
	}
}

// WriteEntry writes a new entry to the footer
func (fw *FooterTabwriter) WriteEntry(key, msg string, args ...any) error {
	_, err := fmt.Fprintf(fw.Writer, key+"\t: "+msg+"\n", args...)
	return err
}

// Flush flushes the footer writer
func (fw *FooterTabwriter) Flush() error {
	return fw.Writer.Flush()
}

// FooterPrinter allows a type to print to the Footer
type FooterPrinter interface {
	PrintFooter(fw *FooterTabwriter) error
}

/// FooterPrinter implementations

// PrintFooter prints the timespan and duration covered
func (tr TimeRange) PrintFooter(fw *FooterTabwriter) error {
	return fw.WriteEntry("Timespan", "[%s, %s] (%s)",
		tr.First.Format(types.DefaultTimeOutputFormat),
		tr.Last.Format(types.DefaultTimeOutputFormat),
		formatting.Durationable(tr.Last.Sub(tr.First).Round(time.Minute)),
	)
}

// PrintFooter prints the conditions of the query in case they are available
func (q Query) PrintFooter(fw *FooterTabwriter) error {
	if q.Condition == "" {
		return nil
	}
	return fw.WriteEntry("Conditions", q.Condition)
}

// PrintFooter prints the queried hosts summary (total, ok, empty, error)
func (hs HostsStatuses) PrintFooter(fw *FooterTabwriter) error {
	return fw.WriteEntry("Hosts", hs.Summary())
}
