---
date: "2026-04-09T17:26:39+00:00"
researcher: Claude
git_commit: ebac58a
branch: claude/document-research-system-mDvG1
repository: goProbe
topic: "Pluggable Enricher Interface Design: How to enrich query results with additional properties (DNS, pod names, firewall rules)"
tags: [research, codebase, enricher, attributes, results, dns, printer, plugin-system]
status: complete
last_updated: "2026-04-09"
last_updated_by: Claude
---

# Research: Pluggable Enricher Interface Design

**Date**: 2026-04-09T17:26:39+00:00
**Researcher**: Claude
**Git Commit**: ebac58a
**Branch**: claude/document-research-system-mDvG1
**Repository**: goProbe

## Research Question

For the goQuery tool, how can a pluggable Enricher interface be created that works with the Attributes in the Results struct and adds additional properties to it? Examples include:
- Inline resolution of IPs to DNS names via reverse lookup (currently handled by a dedicated data structure in the printer)
- Adding columns such as: associated firewall rules, POD name, etc.
- The built-in resolver would be ReverseDNSLookup in contrib; others plugged in by external consumers (like global-query)
- How would adding new query attributes like "pod" work, triggering an IP -> Pod name resolver?

## Summary

The codebase has a clear pipeline: `Args -> Statement -> Runner.Run() -> Result -> PostProcess() -> Print()`. Enrichment currently lives **only** in the Print() step as a `map[string]string` IP->domain mapping. The `Result.Rows[].Attributes` struct is a fixed 4-field struct (`SrcIP`, `DstIP`, `IPProto`, `DstPort`) with no extensibility. The existing plugin system (`plugins/plugin.go`) handles Querier and Resolver registration but has no enricher concept. The `PostProcessor` function type (`func(context.Context, *Result) error`) is the closest existing extension point that could accommodate enrichment. There are several architectural seams where an Enricher could be inserted, and the existing patterns in the codebase (functional options, plugin registration, post-processors) provide clear idioms to follow.

## Detailed Findings

### 1. Current Result Data Structures

The `Result` struct is the central output of every query (`pkg/results/result.go:28-46`):

```go
type Result struct {
    Hostname      string
    Status        Status
    HostsStatuses HostsStatuses
    Summary       Summary
    Query         Query
    Rows          Rows          // []Row
    err           error
}
```

Each `Row` (`pkg/results/result.go:141-150`) contains:

```go
type Row struct {
    Labels     Labels      // timestamp, iface, hostname, host_id
    Attributes Attributes  // sip, dip, proto, dport
    Counters   types.Counters // bytes/packets in/out
}
```

The `Attributes` struct (`pkg/results/result.go:165-170`) is **fixed**:

```go
type Attributes struct {
    SrcIP   netip.Addr `json:"sip,omitempty"`
    DstIP   netip.Addr `json:"dip,omitempty"`
    IPProto uint8      `json:"proto,omitempty"`
    DstPort uint16     `json:"dport,omitempty"`
}
```

This struct has no map, slice, or interface field for additional enrichment data. Any enrichment today happens **outside** the Row/Attributes, as a side-channel (`map[string]string`) passed to the printer.

### 2. Current Attribute Type System

Attributes are defined in `pkg/types/columns.go`. The `Attribute` interface (`line 97-111`) is deliberately closed:

```go
type Attribute interface {
    fmt.Stringer
    Column              // Name() string, Width() Width
    Resolvable() bool
    attributeMarker()   // unexported - prevents external implementations
}
```

Only 4 concrete types exist: `SIPAttribute`, `DIPAttribute`, `ProtoAttribute`, `DportAttribute`. They are registered via a hardcoded `NewAttribute()` switch statement (`lines 224-238`).

The `Resolvable() bool` method already marks whether an attribute can be DNS-resolved (true for SIP/DIP, false for Proto/Dport). This is checked at `pkg/types/columns.go:365-372` via `HasDNSAttributes()`.

### 3. Current DNS Enrichment Flow

DNS resolution is the **only** enrichment that exists today. Its flow:

1. **Statement.Print()** (`pkg/query/query.go:55-150`): After query execution and post-processing, this method checks if DNS is enabled and if any attributes are `Resolvable()` (lines 61-71).

2. **IP Collection** (lines 76-85): Iterates over result rows (up to `MaxRows`), collecting IP strings from `SrcIP` and/or `DstIP` attributes.

3. **Lookup** (line 88): Calls `dns.TimedReverseLookup()` (`pkg/query/dns/dns.go:36-85`), which performs concurrent `net.LookupAddr()` calls with a timeout, returning `map[string]string` (IP -> domain).

4. **Printer Option** (line 91): The mapping is passed to the printer via `results.WithIPDomainMapping()` (`pkg/results/TablePrinter.go:339-344`), stored in `PrinterConfig.ipDomainMapping`.

5. **Display** (`pkg/results/TablePrinter.go:162-216`): During `extract()`, the `tryLookup()` function (lines 151-156) replaces IP strings with domain names when available (lines 181, 183).

Key characteristics:
- DNS resolution is **output-only** - it does not modify `Result.Rows`
- It uses a **side-channel** (`map[string]string`) rather than modifying `Attributes`
- It is tightly coupled to the `Statement.Print()` method
- It has its own config struct `DNSResolution` (`pkg/query/args.go:197-206`) embedded in `Statement`
- There is no caching between queries

### 4. PostProcessor Extension Point

The `PostProcessor` type (`pkg/results/post_process.go:6`):

```go
type PostProcessor func(context.Context, *Result) error
```

Currently used only for time binning (`pkg/query/query.go:17-52`). Post-processors run **after** query execution but **before** printing, and they modify `Result` in-place. This is the most natural existing extension point for enrichment.

### 5. Plugin System Architecture

The plugin system (`plugins/plugin.go:1-54`) uses a singleton registry pattern with `init()` auto-registration:

```go
type Initializer struct {
    sync.RWMutex
    queriers  map[string]QuerierInitializer
    resolvers map[string]ResolverInitializer
}
```

Two plugin types exist:
- **Querier plugins** (`plugins/querier.go`): `QuerierInitializer func(ctx, cfgPath) (distributed.Querier, error)`
- **Resolver plugins** (`plugins/resolver.go`): `ResolverInitializer func(ctx, cfgPath) (hosts.Resolver, error)`

Both follow the same pattern:
- `Register<Type>(name, initFn)` called from `init()` in plugin packages
- `Init<Type>(ctx, name, cfgPath)` to instantiate
- `GetAvailable<Type>Plugins()` to list available plugins
- External plugins are imported via blank imports in `cmd/global-query/cmd/server.go` (e.g., `_ "github.com/els0r/goProbe/plugins/contrib/v4"`)

### 6. Printer Architecture

The `TablePrinter` interface (`pkg/results/TablePrinter.go:272-277`):

```go
type TablePrinter interface {
    AddRow(row Row) error
    AddRows(ctx context.Context, rows Rows) error
    Footer(ctx context.Context, result *Result) error
    Print(result *Result) error
}
```

The `basePrinter` struct (`lines 281-299`) holds the `ips2domains map[string]string` and the list of `OutputColumn` values. Columns are determined by the `columns()` function (`lines 77-135`), which maps `LabelSelector` flags and `Attribute` objects to `OutputColumn` enum values.

The `extract()` function (`lines 162-216`) is the central dispatch that maps `OutputColumn` to a string value from the `Row`. It is a hardcoded switch statement matching each `OutputColumn` variant.

The `PrinterConfig` (`lines 318-333`) uses functional options (`PrinterOption func(*PrinterConfig)`):
- `WithIPDomainMapping()` - DNS resolution
- `WithQueryStats()` - detailed stats

### 7. Query Execution Pipeline (End-to-End)

```
CLI Args (cmd/goQuery/cmd/root.go)
    |
    v
query.Args.Prepare() -> query.Statement    (pkg/query/args.go:585-634)
    |
    v
query.Runner.Run(ctx, args) -> *results.Result    (pkg/query/runner.go:10-14)
    |                                              (implementations: engine.QueryRunner, distributed.QueryRunner)
    v
Statement.PostProcess(ctx, result) -> modifies Result in-place    (pkg/query/query.go:17-52)
    |                                 (currently: time binning, row limiting)
    v
Statement.Print(ctx, result, opts...) -> formatted output    (pkg/query/query.go:55-150)
    |                                    (DNS resolution happens here)
    v
TablePrinter.AddRows() -> TablePrinter.Footer() -> TablePrinter.Print()
```

### 8. How the Key->Row Transformation Works

In `pkg/goDB/engine/query.go:302-379`, raw binary keys are decoded into `Row` objects:

```go
// For each aggregated flow:
rs[count].Attributes.SrcIP = types.RawIPToAddr(key.Key().GetSIP())
rs[count].Attributes.DstIP = types.RawIPToAddr(key.Key().GetDIP())
rs[count].Attributes.IPProto = key.Key().GetProto()
rs[count].Attributes.DstPort = types.PortToUint16(key.Key().GetDport())
```

The underlying `Key` type (`pkg/types/keyval.go:23`) is a fixed byte layout encoding exactly 4 attributes (SIP, DIP, DPort, Proto). The database column indices are also hardcoded (`pkg/types/columns.go:31-44`).

### 9. Distributed Query Result Merging

In `cmd/global-query/pkg/distributed/query.go:210-326`, results from multiple hosts are aggregated via `RowsMap` (`pkg/results/result.go:447`), which is keyed by `MergeableAttributes` (a struct embedding `Labels` + `Attributes`). This merging logic would need to be aware of any enrichment data to avoid losing it during aggregation.

### 10. External Consumer Pattern (global-query)

Global-query (`cmd/global-query/`) uses the library as follows:
1. Creates a `distributed.QueryRunner` wrapping a `Querier` plugin and `ResolverMap`
2. Calls `Run(ctx, args)` which internally resolves hosts, fans out queries, and aggregates results
3. The final result goes through `stmt.PostProcess()` and `stmt.Print()` like local queries

External consumers import plugins via blank imports and configure them via YAML/Viper.

## Code References

- `pkg/results/result.go:28-46` - Result struct definition
- `pkg/results/result.go:141-170` - Row, Labels, Attributes structs
- `pkg/results/result.go:440-447` - RowsMap and MergeableAttributes (merging key)
- `pkg/results/post_process.go:6` - PostProcessor type
- `pkg/results/TablePrinter.go:77-135` - columns() function mapping attributes to output columns
- `pkg/results/TablePrinter.go:151-156` - tryLookup() DNS helper
- `pkg/results/TablePrinter.go:162-216` - extract() central column dispatch
- `pkg/results/TablePrinter.go:272-277` - TablePrinter interface
- `pkg/results/TablePrinter.go:318-344` - PrinterConfig and PrinterOptions
- `pkg/types/columns.go:97-111` - Attribute interface (closed with unexported marker)
- `pkg/types/columns.go:224-238` - NewAttribute() factory (hardcoded switch)
- `pkg/types/columns.go:328-361` - ParseQueryType() tokenization
- `pkg/types/columns.go:365-372` - HasDNSAttributes()
- `pkg/query/query.go:17-52` - PostProcess() method
- `pkg/query/query.go:55-150` - Print() method (DNS resolution happens here)
- `pkg/query/args.go:197-206` - DNSResolution config struct
- `pkg/query/dns/dns.go:36-85` - TimedReverseLookup()
- `pkg/query/runner.go:10-14` - Runner interface
- `pkg/query/statement.go:12-55` - Statement struct
- `pkg/goDB/engine/query.go:302-379` - Key-to-Row transformation
- `pkg/goDB/Query.go:26-68` - Query struct with column indices
- `plugins/plugin.go:1-54` - Plugin system singleton registry
- `plugins/querier.go:1-67` - Querier plugin registration pattern
- `plugins/resolver.go:1-130` - Resolver plugin registration pattern
- `cmd/global-query/pkg/distributed/query.go:54-151` - Distributed QueryRunner

## Architecture Documentation

### Current Patterns and Conventions

1. **Closed Type System for Attributes**: The `Attribute` interface uses an unexported `attributeMarker()` method to prevent external implementations. All 4 attribute types are hardcoded. This is a deliberate design choice tying attributes to on-disk storage columns.

2. **Side-Channel Enrichment**: DNS resolution is implemented as a `map[string]string` passed alongside (not inside) result data. The mapping is passed via functional options to the printer.

3. **Plugin Registration via init()**: The existing plugin system uses Go's `init()` auto-registration with a singleton registry. Plugins register themselves and are activated via blank imports.

4. **Functional Options**: `PrinterOption func(*PrinterConfig)` is the pattern used for configuring the printer.

5. **PostProcessor Pipeline**: A `[]PostProcessor` slice is built and executed sequentially. Currently only has time binning. This is the most natural place to inject enrichment into the pipeline.

6. **Fixed Binary Key Layout**: The database keys encode exactly `{SIP, DIP, DPort, Proto}`. New "attributes" like "pod" cannot be stored in the database - they must be derived/enriched at query time.

7. **Separation of Core vs Derived Data**: Labels (timestamp, iface, hostname, host_id) are already "derived" at query time (not stored per-flow in the key). This establishes a precedent for non-stored data appearing in results.

### Key Architectural Constraints

1. **Attributes struct is fixed**: Adding fields to `Attributes` would change the core data model and affect serialization, merging (`MergeableAttributes`), sorting (`Less()`), and JSON output.

2. **OutputColumn enum is hardcoded**: Adding new output columns requires extending the enum and the `extract()` switch statement.

3. **DNS enrichment is coupled to Print()**: The enrichment logic is embedded in `Statement.Print()`, not abstracted behind an interface.

4. **Merging uses MergeableAttributes as map key**: Any enrichment data that should survive merging must either be part of this key or be re-derivable after merging.

5. **The Attribute interface is closed**: External packages cannot create new `Attribute` implementations. New "pseudo-attributes" like "pod" would need a different mechanism.

### Seams for Enricher Integration

Based on the current architecture, there are several natural insertion points:

1. **PostProcessor stage** (`pkg/query/query.go:17-52`): Enrichers could be registered as PostProcessors that add data to a new extensible field on `Row` or `Result`. This runs after query execution but before printing.

2. **PrinterOption stage** (`pkg/results/TablePrinter.go:335-351`): Like `WithIPDomainMapping()`, additional enrichment mappings could be passed as printer options.

3. **Plugin system** (`plugins/plugin.go`): A new `enricherPlugin` type could be added alongside `querierPlugin` and `resolverPlugin`, following the exact same `init()` registration pattern.

4. **New extensible field on Row**: A `map[string]string` or `map[string]any` field on `Row` (e.g., `Extra` or `Enrichments`) would allow arbitrary enrichment data without changing the fixed `Attributes` struct.

5. **Attribute resolution layer**: A new concept between "query attribute" (what you group by from the database) and "enriched attribute" (derived from query attributes at display time) would separate concerns cleanly.

### Data Flow for a Hypothetical "pod" Attribute

Given the current architecture, a query like `goquery -i eth0 "sip,pod"` would need to:

1. **Parse**: `ParseQueryType("sip,pod")` would need to recognize "pod" as a special enriched attribute (not a database column).
2. **Query**: The database query would run with just `sip` (the actual stored attribute).
3. **Enrich**: After results come back, an enricher would map each `SrcIP` -> pod name.
4. **Display**: The printer would need to know about the "pod" column and extract enriched data.

This is architecturally similar to how DNS resolution already works, but generalized.

## Open Questions

1. **Should enrichment data live on the Row or be a side-channel?** The DNS approach uses a side-channel (`map[string]string` in the printer). For multiple enrichers, should there be a single extensible `map[string]map[string]string` on each Row, or should each enricher provide its own side-channel?

2. **How should enriched attributes interact with merging?** In distributed queries, `RowsMap` merges by `MergeableAttributes`. Enrichment data derived from IPs would need to be re-derived after merging, or the enrichment step must happen after aggregation.

3. **Should enriched attributes be queryable in conditions?** E.g., `goquery -i eth0 "sip,pod" -c "pod=my-pod"`. This would require the enricher to participate in the filter stage, not just post-processing.

4. **How should the attribute parser distinguish database attributes from enriched attributes?** The closed `Attribute` interface prevents external types. A parallel "EnrichedAttribute" concept could coexist.

5. **Should enrichers be registered per-attribute or globally?** E.g., does registering a "pod" enricher automatically make "pod" a valid query attribute, or are they separate registrations?

6. **How should enricher errors be handled?** DNS resolution currently fails silently (IP is shown instead of domain). Should this be the default for all enrichers?

7. **What is the performance contract?** DNS resolution has an explicit timeout and MaxRows limit. Should all enrichers have similar constraints?
