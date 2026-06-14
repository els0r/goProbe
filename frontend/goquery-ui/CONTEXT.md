# goquery-ui

Frontend for exploring goProbe network flow data: composes Queries against the global-query API and visualizes the resulting flows as tables and graphs.

## Language

**Query**:
A parameter set describing which flow data to fetch — time range, interfaces, hosts, attributes, condition.
_Avoid_: search, request, filter

**Run**:
One execution of a Query; starting a new Run cancels and replaces (supersedes) any Run in flight.
_Avoid_: fetch, refresh, execution

**Validation**:
A backend round-trip checking that a Query is well-formed; happens standalone (editor feedback) or as the first phase of a Run.
_Avoid_: linting, client-side check

**Host Error**:
A failure reported by one host of a fanned-out Run; non-fatal — the Run continues with results from the remaining hosts.
_Avoid_: stream error

**Detail Run**:
One execution of a derived Query that populates the details panel for a selected row, IP, host, or interface. Independent of the Run — not a kind of Run. Starting a new Detail Run supersedes the one in flight.
_Avoid_: detail fetch, panel query

**Drill-down**:
A deeper look into one time bucket of the temporal details panel; derives its Query from the Detail Run's target plus the bucket's time window. A new Drill-down supersedes the one in flight.
_Avoid_: drill query, bucket query

**Flow**:
One observed traffic relationship — a source/destination/port/protocol tuple with its inbound and outbound byte and packet counters. One row of a Run's results (`FlowRecord` in code).
_Avoid_: connection, record, row

**Run Total**:
The aggregate in+out volume of a Run, reported in its Summary; the denominator for every per-Flow share.
_Avoid_: grand total, sum

**Run Share**:
One Flow's in+out volume as a percentage of the Run Total. Distinct from the temporal panel's _peak-bucket intensity_ (a bucket vs the busiest bucket) and from _bar geometry_ (sqrt-compressed fill) — neither of those is a share of the Run.
_Avoid_: percent, ratio

## Relationships

- A **Run** executes exactly one **Query**
- At most one **Run** is in flight at any time; a new **Run** supersedes the previous one
- A **Detail Run** derives its Query from the committed Query of its **Run** plus a selected target; it never streams and is never Validated standalone
- A **Detail Run** addresses hosts by the identifiers its **Run** already resolved; it never re-resolves them
- A **Run** and a **Detail Run** may be in flight simultaneously; each concept has its own single slot and its own supersession
- A **Detail Run** belongs to the **Run** whose results it details: starting a new **Run** closes the details panel and supersedes any **Detail Run** in flight
- A **Drill-down** belongs to a **Detail Run**; closing the panel or starting a new **Detail Run** discards it (supersession cascades Run → Detail Run → Drill-down)

## Example dialogue

> **Dev:** "If the user hits Run twice quickly, do we show both results?"
> **Domain expert:** "No — the second **Run** supersedes the first. There is one result slot; a superseded **Run**'s late events must never reach it."
>
> **Dev:** "One of the fanned-out hosts timed out — is the **Run** failed?"
> **Domain expert:** "No, that's a **Host Error**; the **Run** continues and reports it alongside the results. Only losing the connection itself fails the **Run**."

## Flagged ambiguities

- "stream error" conflated two distinct things — resolved: a connection-level failure is a fatal Run failure; a per-host failure is a **Host Error** and never ends the Run.
- "share" was overloaded across three meanings — resolved: **Run Share** is a Flow's % of the **Run Total**; the temporal heatmap's bucket-vs-peak ratio and the diverging-bar sqrt fill are deliberately _not_ called shares.
