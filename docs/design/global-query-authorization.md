# global-query: pluggable authorization and host scoping

Date: 2026-10-03
Status: research notes, no code changed

## Problem

A caller of the `global-query` API must only receive results for hosts it is
entitled to see. The authorization decision (token → entitled hosts) must come
from an external, deployment-specific provider that is plugged in at build or
deploy time, the same way resolvers and queriers are plugged in today.

Two enforcement variants were asked about:

1. hand the entitlement to the querier before fan-out (filter the host list)
2. filter the aggregated results after the fact

## Current state

There is no authentication or authorization anywhere in the server stack.

| Where | What exists | State |
|---|---|---|
| `pkg/api/server/server.go:40` | `DefaultServer.keys []string` with `// TODO: authorize API access` | dead field, never set, never read |
| `cmd/goProbe/cmd/root.go:385` | `api.WithKeys(config.API.Keys)` | commented out; `api.WithKeys` does not exist |
| `cmd/goProbe/config/config.go:493` | `checkKeyConstraints` validates key length and demo keys | config validation only, nothing consumes the keys |
| `pkg/api/client/client.go:236` | client sends `Authorization: digest <key>` via `httpc.AuthToken` | sent, never verified by any server |
| `pkg/api/server/server.go:151` | CORS allow-list includes `Authorization` | header is permitted through, nothing reads it |
| `frontend/goquery-ui/src/api/client.ts:83,128,218` | browser `fetch` with content-type and accept only | no credential or token is sent |
| `frontend/goquery-ui/deploy/Caddyfile:62` | `reverse_proxy /_query*` | forwards request headers to global-query unchanged, including any `Authorization` or cookie an upstream gateway sets |

Consequence: any identity-bearing header reaching global-query today is
trusted-by-absence. Whatever is built must assume the header can be spoofed
unless the network path guarantees otherwise.

## Request path for a distributed query

```
HTTP request
  └ gin middlewares                         pkg/api/server/server.go:registerMiddlewares
      Recovery, CORS, otelgin, TraceID, RequestLogging, RecursionDetector
  └ huma operation middlewares              pkg/api/globalquery/server/server.go:registerRoutes
      RateLimitMiddleware (only if enabled)
  └ handler                                 pkg/api/query.go
      getBodyQueryRunnerHandler / getSSEBodyQueryRunnerHandler
      → prepareArgs(ctx, caller, args)      sets defaults, logs args at Info
      → QueryRunner.Run / RunStreaming
  └ QueryRunner.run                         cmd/global-query/pkg/distributed/query.go
      1. QueryHosts must be non-empty
      2. pick resolver by args.QueryHostsResolverType (default "string")
      3. args.Prepare(), semaphore, CheckUnboundedQueries
      4. prepareHostList(ctx, resolver, args.QueryHosts)
           "*"  → querier.(QuerierAnyable).AllHosts()      no ctx, no filter
           else → resolver.Resolve(ctx, queryHosts)
      5. querier.Query(ctx, hostList, &queryArgs)          fan-out, one remote query per host
      6. aggregateResults(ctx, stmt, resChan, send)       merge rows, union HostsStatuses
```

Context propagation is intact end to end. `humagin` builds the huma context
from `c.Request.Context()` (`humagin.go:54`), the handler receives
`ctx.Context()` for both plain and SSE operations (`sse.go:215`), and
`QueryRunner.run` passes `ctx` into `resolver.Resolve` and `querier.Query`.
Any value placed on the request context by either a gin middleware (pattern:
`TraceIDMiddleware`, `pkg/api/middleware.go:37`) or a huma operation
middleware (`huma.WithValue`, `huma/v2/api.go:163`) is visible to the runner,
the resolver and the querier without further plumbing.

## Where host identity exists along the path

Three granularities, with very different cost and reliability for filtering.

### 1. Host list, after resolve and before fan-out

`hosts.Hosts` is `[]hosts.ID`, `ID = string`
(`pkg/distributed/hosts/resolver.go:11`). For the API client querier the ID is
the key in the endpoints YAML (`hostA:` in
`examples/config/global-query-api-client-querier-example-config.yaml`), looked
up in `APIClientQuerier.APIEndpoints[host]`
(`plugins/querier/apiclient/querier.go:86`).

This is the only place where the identifier is the one the operator chose and
the only place where dropping a host costs nothing downstream. Filtering here
removes the remote query, the network round trip, the semaphore slot and the
aggregation work for every disallowed host.

### 2. Per-host result, before merge

`APIClientQuerier.Query` sets `qr.Hostname = wl.Host` (the endpoint key) on
each result before putting it on the channel (`querier.go:176`).
`aggregateSingleResult` could drop a result by `qr.Hostname` before
`rowMap.MergeRows`. Correct, but the remote query has already run.

### 3. Final aggregated rows

`Row.Labels.Hostname` and `Row.Labels.HostID` are set unconditionally by the
sensor from `os.Hostname()` and the goDB host ID
(`pkg/goDB/engine/query.go:182-187,342-343`). Two problems:

- The label is the sensor's self-reported hostname, not the endpoint key. The
  two can differ (`hostA:` in the config can point at a box whose
  `os.Hostname()` is `sensor-01.prod`). A filter on rows therefore needs a
  second mapping that nothing in the codebase maintains.
- Summary fields (`Totals`, `Hits`, `Interfaces`, `Stats`) are additive scalars
  with no per-host breakdown (`query.go:285-290`). They cannot be corrected
  after a row is dropped.

Row-level filtering is not viable without changing the result model.

## Extension points that already exist

- **Plugin registry** `plugins/plugin.go`: a singleton with two plugin kinds
  (`querier`, `resolver`), `init()`-time registration via blank import,
  `panic` on duplicate names, `InitX(ctx, name, cfgPath)` constructors, and an
  out-of-tree hook through the `goprobe_contrib` build tag
  (`plugins/contrib/gen.go` generates `import _ "github.com/els0r/goProbe-contrib"`).
  A third kind slots in with the same shape.
- **Config surface** `cmd/global-query/pkg/conf/conf.go`: viper keys
  `hosts.resolver.type/config`, `querier.type/config`, `plugins.AppConfig` for
  the list form. `cmd/global-query/cmd/init_plugins.go` wires them.
- **Operation-level middleware** `huma.Middlewares` passed into
  `api.RegisterQueryAPI` and attached to the `/_query` and `/_query/sse`
  operations only (`pkg/api/query_api_ops.go:112,128`). Info routes are
  exempt by construction. The rate limiter is the existing example.
- **Request-scoped values** via `huma.WithValue` / `c.Request.WithContext`,
  as shown above.
- **`QueryRunner` options** (`gqdistributed.QueryOption`,
  `WithMaxConcurrent`) for injecting a dependency into the runner without
  changing `NewQueryRunner`'s signature.

## Gaps and sharp edges found

1. `QuerierAnyable.AllHosts()` takes no `ctx` (`pkg/distributed/querier.go:24`).
   The `*` selector bypasses the resolver entirely, so a resolver-based
   scoping scheme does not cover it.
2. An empty host list is not an error. `prepareHostList` returns an empty
   slice, `APIClientQuerier.Query` spawns zero runners and closes the channel,
   and the caller gets an empty `ok`/`empty` result instead of 403.
3. `HostsStatuses` in the final result enumerates every host that was queried
   (`query.go:261`). Hosts the caller is not entitled to must never appear
   there, including as error entries.
4. Unknown host names are echoed back: `createQueryWorkload` injects an
   `ErrorRunner` whose message lands in `HostsStatuses[host]`
   (`querier.go:86-92`). A caller can probe fleet membership by name. With
   pre-dispatch filtering this disappears for disallowed hosts, since they
   never reach the querier.
5. `query.Args` is logged at Info in `prepareArgs` and attached verbatim to a
   trace span (`query.go:102`). Nothing credential-like may ever be put into
   `Args`.
6. `Authorization` is already forwarded by Caddy and allowed by CORS. The
   browser frontend sends nothing; an upstream gateway could.
7. The caller selects the resolver via `args.QueryHostsResolverType`. Any
   scheme where only one resolver is "the safe one" is opt-in and therefore
   not an authorization boundary.
8. `goProbe` (the sensor) shares `DefaultServer` and `RegisterQueryAPI`. An
   authentication hook placed on `DefaultServer` would be reusable for the
   sensor API; host scoping is global-query-only.

## Options

### Option A: authorizer plugin kind, scope enforced in `QueryRunner` before fan-out

Add a third plugin kind next to querier and resolver.

```go
// pkg/distributed/authz/authz.go (new)

// Scope is what a principal may see. It is opaque to global-query except
// for its ability to filter host IDs.
type Scope interface {
    // Filter returns the subset of hosts the principal may query.
    Filter(ctx context.Context, in hosts.Hosts) (hosts.Hosts, error)
}

// Authorizer turns an incoming request into a Scope.
// Implementations own token validation, caching and the mapping from
// tenant/principal to hosts.ID values.
type Authorizer interface {
    Authorize(ctx context.Context, r *http.Request) (Scope, error)
}
```

```go
// plugins/authorizer.go (new, mirrors plugins/resolver.go)
type AuthorizerInitializer func(ctx context.Context, cfgPath string) (authz.Authorizer, error)
func RegisterAuthorizer(name string, initFn AuthorizerInitializer)
func InitAuthorizer(ctx context.Context, name, cfgPath string) (authz.Authorizer, error)
```

Wiring:

- `conf.go`: `authz.type`, `authz.config` keys and flags.
- `init_plugins.go`: `initAuthorizer(ctx)`; nil when unconfigured.
- `pkg/api/middleware.go`: `AuthorizationMiddleware(a authz.Authorizer) huma.Middleware`
  that calls `Authorize`, returns 401/403 via `huma.WriteErr` on error, and
  stores the `Scope` on the context with `huma.WithValue`. It is appended to
  the same `huma.Middlewares` slice as the rate limiter in
  `pkg/api/globalquery/server/server.go:registerRoutes`, so it covers
  `/_query` and `/_query/sse` and nothing else.
- `cmd/global-query/pkg/distributed/query.go`: in `prepareHostList`, after
  both the `*` branch and the resolver branch, read the `Scope` from `ctx` and
  apply `Filter`. If the result is empty, return a typed error that the
  handler maps to 403.
- `aggregateResults` is untouched: disallowed hosts never reach it, so
  `HostsStatuses`, totals and hit counts are correct by construction.

Behaviour when no authorizer is configured is a decision point (see below).

Properties:

- Covers `*`, every resolver type, plain and SSE routes, in one place.
- Zero cost for disallowed hosts. No change to the result model.
- Pluggable in the same way as resolvers: a company builds a package that
  calls `plugins.RegisterAuthorizer("acme-iam", ...)` and imports it through
  the contrib hook or its own `main`.
- Small surface: one new interface file, one plugin registry file, one
  middleware, a dozen lines in `prepareHostList`, config keys. The existing
  `mockResolver`/`mockQuerier` tests in `query_test.go` extend naturally with
  a `mockScope`.

Weak spots:

- `Filter` works on an enumerated list. For `*` on a 10k-host fleet it is an
  O(n) pass per request, which is still far cheaper than one remote query per
  host. If a provider can enumerate a tenant's fleet directly, an optional
  `Hosts() (hosts.Hosts, bool)` on `Scope` lets `*` short-circuit; not needed
  for a first version.
- Token verification latency is on the request path. Caching is the plugin's
  job; the interface should say so.

### Option B: authorization as a resolver

Register a resolver that ignores the host query and returns the principal's
fleet, selected with `query_hosts_resolver_type: tenant`.

This is the "give the querier the information" variant in its purest form,
and it fits the existing plugin system with no new kind. It fails as an
authorization boundary on its own because of gaps 1 and 7: the caller picks
the resolver, and `*` never consults one. It is a fine convenience layer on
top of Option A (a tenant resolver that reads the same `Scope` from `ctx` and
returns `scope.Hosts()`), not a replacement.

### Option C: filter aggregated results

Drop per-host results in `aggregateSingleResult` by `qr.Hostname`, or drop
rows by `Row.Labels.Hostname` in `finalizeResult`.

Per-host filtering before merge is correct but pays the full remote query for
every disallowed host, holds semaphore slots for them, and still needs the
scope on `ctx`, so it is strictly more expensive than Option A for the same
plumbing. Row-level filtering is incorrect for the two reasons in section
"Final aggregated rows": the label is the sensor's own hostname, and summary
counters cannot be rebuilt. Not recommended in either form.

### Option D: external enforcement, global-query trusts a header

An API gateway or sidecar (OPA, Envoy `ext_authz`, an IdP-aware ingress)
validates the token and injects an allow-list header such as
`X-Allowed-Hosts: hostA,hostB` or a signed JWT with a `hosts` claim.
global-query parses it and filters.

This is not a separate architecture. It is one `Authorizer` implementation
under Option A (`authz.type: header`), and probably the first one worth
shipping in-tree because it needs no external SDK. It is only safe when the
network guarantees the header cannot be set by the client, which is not the
case through the current Caddy proxy without an upstream gateway stripping
and re-setting it. The in-tree plugin should refuse to start unless the
operator explicitly acknowledges that (a `trusted: true` config key or
similar).

### Option E: token or tenant inside `query.Args`

Rejected. `Args` is logged at Info and attached to trace spans (gap 5), is
part of the public OpenAPI schema, and travels unchanged to every sensor.

## Comparison

| | A: authorizer plugin, pre-dispatch | B: tenant resolver | C: post-query filter | D: trusted header |
|---|---|---|---|---|
| Covers `*` selector | yes | no | yes | yes (as A impl) |
| Enforced regardless of caller input | yes | no | yes | yes (as A impl) |
| Remote work for disallowed hosts | none | none | full | none |
| Correct `HostsStatuses` and totals | by construction | by construction | per-host: yes, row-level: no | by construction |
| Pluggable provider | yes, new plugin kind | yes, existing kind | needs scope anyway | one plugin of A |
| Code touched | 5 files + tests | 1 plugin | aggregation internals | 1 plugin |
| SSE route covered | yes | yes | yes | yes |

## Recommendation

Option A, with Option D as the first in-tree `Authorizer` implementation and
Option B as an optional convenience resolver layered on top. The identity is
extracted once per request in an operation-level huma middleware, carried on
the context, and applied exactly once in `prepareHostList`. Nothing below
that point learns about authorization.

## Decisions needed before implementation

1. **Fail-open or fail-closed when `authz.type` is unset.** Today everything
   is open. Fail-closed by default is the safer contract but breaks every
   existing deployment on upgrade; fail-open with a startup warning preserves
   behaviour. Proposal: fail-open with a loud warning now, flip the default
   in the next major.
2. **Response when the filtered host list is empty.** 403 with a problem
   detail, or 200 with an empty result. 403 is more honest and lets the
   frontend show something useful.
3. **Response when some requested hosts were filtered.** Silently narrow (the
   user story as written), or 200 plus a `forbidden` status per dropped host
   in `HostsStatuses`. The latter leaks that the host exists. Proposal:
   silently narrow.
4. **Scope shape.** `Filter(hosts) hosts` only, or also an enumerating
   `Hosts()` for cheap `*`. Proposal: `Filter` only; add `Hosts()` when a
   provider needs it.
5. **Where token verification runs.** In the plugin (self-contained, portable)
   or upstream with a trusted header (Option D). Both are the same interface;
   the choice is per deployment.
6. **Reuse for goProbe's own API.** An authentication-only hook on
   `DefaultServer` would resurrect the dead `keys` field. Out of scope for
   host scoping, but the middleware placement should not preclude it.

## Files that change under Option A

| File | Change |
|---|---|
| `pkg/distributed/authz/authz.go` | new: `Authorizer`, `Scope`, context helpers, `ErrNoAuthorizedHosts` |
| `plugins/authorizer.go` | new: registry functions mirroring `plugins/resolver.go` |
| `plugins/plugin.go` | add `authorizers` map to `Initializer`, extend `LogValue`/`GetAvailablePlugins` |
| `plugins/authorizer/header/` | new: in-tree trusted-header implementation (Option D) |
| `pkg/api/middleware.go` | new `AuthorizationMiddleware` |
| `pkg/api/globalquery/server/server.go` | accept an `authz.Authorizer`, append middleware |
| `cmd/global-query/pkg/distributed/query.go` | apply scope in `prepareHostList`, map empty result to 403 |
| `pkg/distributed/querier.go` | optional: `AllHosts(ctx)` so a scope-aware querier can short-circuit `*` |
| `cmd/global-query/pkg/conf/conf.go`, `cmd/global-query/cmd/init_plugins.go`, `server.go` | config keys, init, wiring |
| `cmd/global-query/README.md`, `examples/config/` | document `authz` block |
| tests | `query_test.go` with a `mockScope`; `plugins/authorizer_test.go`; middleware test |
