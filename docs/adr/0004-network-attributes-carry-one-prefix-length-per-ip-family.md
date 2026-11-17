# 4. Network attributes carry one prefix length per IP family and exclude their IP attribute

Date: 2026-11-17

## Status

Accepted. Implementation tracked in PRD #522.

## Context

Grouping flows by network segment needs a prefix length, and the database
stores IPv4 and IPv6 flows side by side in every block, so one length cannot
describe both families. The query-type grammar is public interface (CLI, API,
and the result echo), so whatever is chosen is hard to walk back.

Terms (**Network attribute**, **Prefix length**, **canonical form**) are
defined in [CONTEXT.md](../../CONTEXT.md).

Three grammars were weighed:

1. **One length for both families** (`snet/24` truncates IPv6 to 24 bits too).
   Rejected: a 24-bit IPv6 bucket has no operational meaning, and the user
   cannot ask for anything else.

2. **One family per attribute, the other dropped** (`snet/24` groups IPv4 and
   silently discards IPv6 rows). Rejected: it mirrors the condition grammar
   but loses data without saying so, and totals would no longer match the
   per-host query.

3. **One length per family, IPv4 first, IPv6 second, IPv6 defaulting to /64**
   (`snet/24`, `snet/24/56`). Chosen.

Two further rules follow from the same concern:

- A bare `snet`/`dnet` is a parse error, not a defaulted length. A guessed
  IPv4 length changes what a row means, so it must be written down.
- `sip` and `snet` (likewise `dip` and `dnet`) cannot appear in one query, nor
  can a network attribute appear twice on one side. Both occupy the same key
  column; allowing both would make the row meaning depend on order.

## Decision

`snet/<v4>[/<v6>]` and `dnet/<v4>[/<v6>]`, IPv6 default 64, bounds 0..32 and
0..128. Results echo the canonical form with both lengths filled in, so the
echo is a valid query type that reproduces the grouping. Masking is applied
when the aggregation key is built, never as a post-aggregation merge, because
the point of the feature is to keep memory proportional to segments, not
hosts.

## Consequences

- The result row gains `snet`/`dnet` prefix fields; `sip`/`dip` stay
  host-only, so no consumer can mistake a segment for a host.
- Live queries are rejected with a network attribute until the live path
  projects keys like the DB reader does (#520).
- A shorthand such as `talk_net` would need a default IPv4 length, which this
  decision refuses to guess; none is added.
