# goProbe — Context

Shared language for the project. Terms here are the canonical names; use them in
code, commits, and discussion. Decisions with lasting trade-offs live in
[docs/adr/](docs/adr/).

## goquery-ui chart releases

The `charts/goquery-ui` Helm chart is released on two distinct **avenues**.
Keep them straight — they bump different fields and fire on different events.

- **Chart-only bump (avenue 1)** — a change to the chart's templates or values.
  Bumps the chart's own `version:` and is published by `publish-chart.yml` on
  merge to `main`. Independent of the app's release tags.

- **Tagged release (avenue 2)** — an app release via a `v*.*.*` git tag. Builds
  a new `goprobe/frontend` image and, in the same pipeline, retargets the
  chart's `appVersion` to that image, patch-bumps `version:`, ships the chart,
  and commits both fields back to `main`. Prerelease tags (`-rc*`) build images
  but do not ship a chart. See ADR 0002.

- **Chart version (`version:`)** — the chart's *own* semver (`0.x` line),
  bumped on both avenues. Distinct from `appVersion`.

- **appVersion** — the `goprobe/frontend` image tag the chart is validated
  against and pins by default. `image.tag` overrides it per deployment. It is
  an exact pin, not a floating tag — installs are reproducible (ADR 0002).

## Distributed query authorization

Language for scoping `global-query` results to what a caller may see. Two
identifiers exist for a sensor and they are not interchangeable.

- **Host ID** — the operator-chosen identifier under which `global-query` knows
  a sensor. The only identifier authorization uses.
  _Avoid_: endpoint key, host name.

- **Hostname** — what a sensor reports about itself; it appears in result rows.
  Never an authorization input.

- **Principal** — the opaque, non-secret identity of the caller. Used for audit
  only, never for the authorization decision.
  _Avoid_: user, tenant, caller ID.

- **Scope** — the set of host IDs one principal may query, established once per
  request.
  _Avoid_: entitlement, allow-list.

- **Authorizer** — the deployment-specific provider that turns a request into a
  scope.
  _Avoid_: auth plugin, IAM.

- **Scoped query** — a query whose host list was narrowed by a scope. An
  **unscoped query** runs when no authorizer is configured.
  _Avoid_: filtered query.

Relationships:

- An **Authorizer** yields exactly one **Scope** per request, or rejects it.
- A **Scope** belongs to one **Principal** and contains zero or more **Host IDs**.
- A **Scoped query** reaches only the **Host IDs** in its **Scope**; the others
  are dropped silently and never appear in the result.

Flagged ambiguities:

- "tenant" was used for a group of principals sharing a fleet — resolved: it is
  not a goProbe concept. How principals group is internal to an **Authorizer**.
- "host" was used for both **Host ID** and **Hostname** — resolved: these are
  distinct namespaces and can differ for the same sensor.

## Query attributes

Language for what a query groups flows by. Attributes are the columns of the
result key; **conditions** are the filter and use a separate grammar even
where the words overlap.

- **Attribute** — a column flows are grouped by (`sip`, `dip`, `proto`,
  `dport`, and the network attributes below). The attribute list is the
  query type.
  _Avoid_: column, field, dimension.

- **Network attribute** — `snet` or `dnet`: the source or destination IP
  truncated to a prefix length, so rows group by network segment instead of
  by host. Written `snet/<v4>/<v6>` with the IPv4 length first and the IPv6
  length second; `snet/24` sets IPv4 to /24 and leaves IPv6 at the default
  /64. A bare `snet` without a length is a parse error, never a guessed
  default. Bounds: 0..32 for IPv4, 0..128 for IPv6.
  _Avoid_: subnet attribute, masked IP.

- **Prefix length** — the number of leading bits kept by a network attribute.
  One per IP family, set independently per attribute (`snet/24,dnet/29`).
  _Avoid_: netmask (reserved for the condition grammar's CIDR value).

Relationships:

- A **Network attribute** occupies the same key column as its IP attribute
  (`snet` ↔ `sip`, `dnet` ↔ `dip`). One attribute per key column: `sip,snet`
  or `snet/24,snet/16` is a parse error, never a silent override.
- `snet`/`dnet` in a **condition** filter on a CIDR; as an **Attribute** they
  group by prefix length. Same word, two grammars, no shared value.
- A result row carries a **Network attribute** as a prefix (`10.0.0.0/24`),
  in its own `snet`/`dnet` field, never as a bare address in `sip`/`dip`.
  Two rows are the same segment only if address and prefix length match.
- A **Network attribute** is never resolved via reverse DNS.
- A result echoes its attributes in **canonical form**: a network attribute
  always as `snet/<v4>/<v6>` with both lengths filled in, so the echo is a
  valid query type that reproduces the grouping.

Flagged ambiguities:

- "mask" was used for both the condition CIDR and the grouping length —
  resolved: **Prefix length** is the grouping term; netmask stays with
  conditions.
