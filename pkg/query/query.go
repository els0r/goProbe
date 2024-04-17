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
func (s *Statement) Print(ctx context.Context, result *results.Result, opts ...results.PrinterOption) error {
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
		ips2domains = dns.TimedReverseLookup(ctx, ips, s.DNSResolution.Timeout)
		result.Summary.Timings.ResolutionDuration = time.Since(resolveStart)

		opts = append(opts, results.WithIPDomainMapping(ips2domains, s.DNSResolution.Timeout))
	}

	cfg := &results.PrinterConfig{
		Format:        s.Format,
		SortOrder:     s.SortBy,
		LabelSelector: s.LabelSelector,
		Direction:     s.Direction,
		Attributes:    s.attributes,
		Totals:        result.Summary.Totals,
		NumFlows:      result.Summary.Hits.Total,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// get the right printer
	printer, err := results.NewTablePrinter(s.Output, cfg)
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
	err = printer.Footer(ctx, result)
	if err != nil {
		if memErr != nil {
			return memErr
		}
		return err
	}

	return printer.Print(result)
}
