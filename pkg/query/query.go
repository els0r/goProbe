package query

import (
	"context"
	"fmt"
	"time"

	"github.com/els0r/goProbe/pkg/query/dns"
	"github.com/els0r/goProbe/pkg/query/heap"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/telemetry/tracing"
)

// Print prints a statement to the result
func (s *Statement) Print(ctx context.Context, result *results.Result) error {
	ctx, span := tracing.Start(ctx, "(*Statement).Print")
	defer span.End()

	var sip, dip types.Attribute

	var hasDNSattributes bool
	for _, attribute := range s.attributes {
		switch attribute.Name() {
		case "sip":
			sip = attribute
			hasDNSattributes = true
		case "dip":
			dip = attribute
			hasDNSattributes = true
		}
	}

	// Find map from ips to domains for reverse DNS
	var ips2domains map[string]string
	if s.DNSResolution.Enabled && hasDNSattributes {
		var ips []string
		for i, l := 0, len(result.Rows); i < l && i < s.DNSResolution.MaxRows; i++ {
			attr := result.Rows[i].Attributes
			if sip != nil {
				ips = append(ips, attr.SrcIP.String())
			}
			if dip != nil {
				ips = append(ips, attr.DstIP.String())
			}
		}

		resolveStart := time.Now()
		ips2domains = dns.TimedReverseLookup(ips, s.DNSResolution.Timeout)
		result.Summary.Timings.ResolutionDuration = time.Since(resolveStart)
	}

	// get the right printer
	printer, err := results.NewTablePrinter(
		s.Output,
		s.Format,
		s.SortBy,
		s.LabelSelector,
		s.Direction,
		s.attributes,
		ips2domains,
		result.Summary.Totals,
		result.Summary.Hits.Total,
		s.DNSResolution.Timeout,
		// TODO: make this a printer config
	)
	if err != nil {
		return err
	}

	// start ticker to check memory consumption every second
	heapWatchCtx, cancelHeapWatch := context.WithCancel(ctx)
	defer cancelHeapWatch()

	memErrors := heap.Watch(heapWatchCtx, s.MaxMemPct)

	printCtx, printCancel := context.WithCancel(ctx)
	defer printCancel()

	var memErr error
	go func() {
		select {
		case memErr = <-memErrors:
			memErr = fmt.Errorf("%w: %w", heap.ErrorMemoryBreach, err)
			printCancel()
			return
		case <-printCtx.Done():
			return
		}
	}()
	err = printer.AddRows(printCtx, result.Rows)
	if err != nil {
		if memErr != nil {
			return memErr
		}
		return err
	}
	err = printer.Footer(result)
	if err != nil {
		if memErr != nil {
			return memErr
		}
		return err
	}

	return printer.Print(result)
}
