# 3. global-query scopes queries by host ID before fan-out, through an authorizer plugin

Date: 2026-10-06

## Status

Accepted

## Context

A caller of the `global-query` API must only receive results for the host IDs
in its scope. Nothing in the server stack authenticates or authorizes today,
and the decision (request → scope) belongs to a deployment-specific provider
that is plugged in the same way queriers and resolvers are.

Terms (**host ID**, **hostname**, **principal**, **scope**, **authorizer**) are
defined in [CONTEXT.md](../../CONTEXT.md).

Five alternatives were weighed:

1. **Scoping as a resolver.** A resolver that returns the caller's hosts fits
   the existing plugin system with no new kind. Rejected: the caller picks the
   resolver through the query arguments, and the `*` selector never consults a
   resolver, so it is opt-in and not a boundary.

2. **Drop per-host results before they are merged.** Correct, but every host
   outside the scope has already been queried and has held a concurrency slot.
   Rejected: same plumbing as option 5 at strictly higher cost.

3. **Drop rows from the aggregated result.** Rejected as incorrect: rows carry
   the sensor's self-reported hostname, not the host ID, and the summary
   totals and hit counts are additive scalars that cannot be rebuilt once a
   host's rows are removed.

4. **Carry the credential in the query arguments.** Rejected: the arguments
   are logged, attached to trace spans, part of the public API schema, and
   forwarded unchanged to every sensor.

5. **A third plugin kind, the authorizer, whose scope narrows the host list
   before fan-out.** Chosen.

## Decision

**Contract.** An authorizer turns a request into a scope, or rejects it. A
scope exposes the principal and a filter over a list of host IDs, nothing
else. The authorizer does not receive the HTTP request itself but a read-only
view of it: headers (all values), TLS connection state and remote address. It
cannot read the body.

**Where it runs.** Authorization is an operation-level middleware on the two
routes that dispatch queries (`/_query`, `/_query/sse`). It runs before the
rate limiter, so rejected requests never consume the shared budget. Validation
and info routes stay open: they never reach a sensor.

**Who enforces.** The query runner, not the middleware. When an authorizer is
configured the runner requires a scope on every query, and a query that
arrives without one fails. It never runs unscoped because a middleware was not
attached.

**How it is applied.** Once, after the host list is resolved (including `*`)
and before fan-out. Host IDs outside the scope are dropped silently and never
appear in the result. A request whose every host is outside the scope is
answered with 403. A scope may only narrow: one that returns a host ID it was
not given is treated as an authorizer failure.

**Failures.** An authorizer signals "no valid credential" (401) or "not
allowed" (403) explicitly. Any other failure, including an unreachable
provider, is answered with 503: authorization never fails open. What the
authorizer reports is logged, never returned to the caller.

**No authorizer configured.** Queries run unscoped and the server warns once
at startup. The next major version flips this to fail-closed.

**First in-tree authorizer.** `header` reads the scope from a request header
set by a trusted gateway. It refuses to start unless the operator explicitly
acknowledges that the network path prevents clients from setting that header,
it has no wildcard, and it rejects a request carrying the header more than
once. A second, optional header supplies the principal for audit.

**Audit.** The principal is attached to every log line of the request from
the point of authorization on. The runner logs how many host IDs were
requested and how many remained; the dropped host IDs only at debug level.

## Consequences

- Hosts outside the scope cost nothing: no remote query, no concurrency slot,
  no aggregation. Per-host statuses and summary totals are correct by
  construction.
- A caller cannot tell a host ID that does not exist from one outside its
  scope. Unknown host IDs inside the scope are still reported as errors.
- The authorizer contract is public: out-of-tree plugins build against it, so
  changing it is a breaking change. The request view can gain methods without
  breaking plugins, since plugins only consume it.
- `*` enumerates the whole fleet and then filters, on every request. If a
  provider can list a principal's hosts directly, a scope can later offer that
  as an optional shortcut without changing the querier interface.
- The `header` authorizer is exactly as safe as the network path in front of
  it, and its scope must fit in a request header, which bounds it to small
  fleets per principal.
- Only `global-query` is scoped. Sensors still answer anyone who can reach
  them, so the path between `global-query` and the sensors must not be
  reachable by callers.
- The access-log line of a request does not carry the principal. Joining it
  to the audit lines needs the trace ID, which exists only when tracing is
  enabled.
- Upgrading changes nothing until an authorizer is configured, and will stop
  serving unscoped queries with the next major version.
