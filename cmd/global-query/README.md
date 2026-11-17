# global-query

> Query server able to aggregate query results across a fleet of hosts

The tool `global-query` is the central entrypoint for running queries across a fleet of hosts. If supplied with configuration on how to reach sensors running `goProbe`, it will be able to run queries on said hosts.

It's the API pendant to `goQuery`. While goQuery is usually used for running queries against a local DB on the same system, on which capturing is performed, `global-query` will aggregate results across a set of hosts.

The query server is meant to be deployed centrally (i.e. in a kubernetes cluster) and should serve as the backend for a front-end displaying results. `goQuery` itself can take the role of that front end and display aggregated results.

## Getting Started

From the repository root:

```sh
go run ./cmd/global-query --config examples/config/global-query-example-config.yaml server
```

## Deployment

`global-query` ships as the `query` target of the repository [Dockerfile](../../Dockerfile) (image `goprobe/query`, entrypoint `global-query server`); see the `query` service in [docker-compose.yaml](../../docker-compose.yaml). The tool is written in pure Go and has no C library dependencies (in contrast to `goProbe`/`goQuery`).

## Distributed Query Runner

Aggregation of results is purely based on the [`results.Results`](../../pkg/results/result.go) data structure.

The query server is set up in a way that different query runners can be used to fetch the results from the sensors. The default distributed query runner is the [`APIClientQuerier`](../../plugins/querier/apiclient/querier.go), which uses the [`goProbe` Client](../../pkg/api/goprobe/client/) to fetch results from a sensor. It is the `api` querier, selected by `querier.type` (the default).

### API Client Querier Configuration

Each distributed query runner will rely on specific configuration. In the case of the API client querier, the `global-query` server needs knowledge on how to reach a given host that was specified in the hosts list.

An example configuration for the API Client Querier is available under [global-query-api-client-querier-example-config.yaml](../../examples/config/global-query-api-client-querier-example-config.yaml).

### Custom Query Runners

Other queriers plug in through the [contrib mechanism](../../plugins/contrib/README.md): implement `distributed.Querier` ([pkg/distributed/querier.go](../../pkg/distributed/querier.go)), register it with `plugins.RegisterQuerier`, and select it with `querier.type` / `querier.config`.

## Configuration

Next to the authorizer keys described below, the configuration file accepts:

| Key                     | Default          | Meaning                                                    |
|-------------------------|------------------|------------------------------------------------------------|
| `server.addr`           | `localhost:8145` | Address to which the server binds                          |
| `server.cors_origins`   | none             | Allowed CORS origins for browser clients (empty: allow none) |
| `hosts.resolver.type`   | `string`         | Resolver used for the hosts resolution query               |
| `hosts.resolvers`       | none             | List of additional resolvers, each with `type` and `config` (path to its configuration file) |
| `querier.type`          | `api`            | Querier used to run queries                                |
| `querier.config`        | `""`             | Querier config file location                               |
| `profiling.enabled`     | `false`          | Enable profiling endpoints                                 |
| `logging.level`         | `info`           | Log level                                                  |
| `logging.encoding`      | `logfmt`         | Message encoding format                                    |
| `logging.destination`   | `""`             | Logging destination file path (empty for stdout)           |

Except for `hosts.resolvers`, every key is also a flag of the same name. The `server.*`, `authorizer.*`, `profiling.enabled` and `openapi.spec-outfile` flags belong to the `server` subcommand and must follow `server` on the command line.

## Authorization

`global-query` can scope every query to the **host IDs** a caller may see. The decision is made by an **authorizer**, a plugin selected the same way a querier is: by type and config file.

| Key                 | Flag                  | Default | Meaning                                                   |
|---------------------|-----------------------|---------|-----------------------------------------------------------|
| `authorizer.type`   | `--authorizer.type`   | `""`    | Name of the registered authorizer plugin. Empty: none     |
| `authorizer.config` | `--authorizer.config` | `""`    | Path to the authorizer's configuration file, if it needs one |

```yaml
authorizer:
  type: my-authorizer
  config: ./my-authorizer.yaml
```

With an authorizer configured, each request to `/_query` and `/_query/sse` is turned into a **scope** before fan-out: host IDs outside the scope are dropped silently, a request whose every host lies outside the scope is answered with `403`, a request without a valid credential with `401`. Any other authorizer or scope-filter failure, including an unreachable provider, is answered with `503`: authorization never fails open. The vocabulary (host ID, principal, scope) is defined in [CONTEXT.md](../../CONTEXT.md), the design in [ADR 0003](../../docs/adr/0003-global-query-scopes-queries-by-host-id-before-fan-out.md).

**No authorizer (the default).** Queries run as **unscoped queries**: every caller reaches every host. The server warns once at startup. This default flips to fail-closed with the next major version, where unscoped operation becomes an explicit opt-in.

**Unknown type.** A type that is not registered fails startup with an error naming it, so a typo cannot silently leave queries unscoped.

**Audit trail.** From the point of authorization on, every log line of a request carries the **principal** as the `principal` field. A scoped query logs at `info` how many host IDs were requested and how many remained after narrowing (`requested`, `remaining`); the dropped host IDs themselves appear at `debug` only, so routine logs do not carry other principals' host names. For the same reason the query arguments of a scoped query are logged with `query_hosts` redacted: the host list as the caller requested it may name hosts outside the scope. The span attributes of a scoped query redact it likewise. A rejected request produces one `warn` line with the authorizer's reason, which is never returned to the caller. The per-request access-log line is written before authorization and does not carry the principal; with tracing enabled, the trace ID joins it to the audit lines, and the host-list span carries the principal as an attribute, also when resolving the host list fails.

**Sensors remain unauthenticated.** Only `global-query` is scoped. A `goProbe` sensor still answers anyone who can reach its API, so the path between `global-query` and the sensors must stay private: it must not be reachable by the callers of `global-query`.

One authorizer ships in-tree, `header`, described below. Out-of-tree authorizers register through the [contrib mechanism](../../plugins/contrib/README.md), which also documents the contract for plugin authors.

### The `header` authorizer

The `header` authorizer reads the **scope** from a request header set by a trusted gateway in front of `global-query`, and optionally the **principal** from a second header. It lets an operator enable scoping without writing code.

> **Warning: the `header` authorizer is exactly as safe as the network path in front of it.** It trusts the scope header unconditionally. The gateway must **replace** the scope and principal headers on **every** path a client can reach, never append to them, and no proxy between the gateway and `global-query` may fold repeated headers into one value. If a client can reach `global-query` directly, or through any path that passes its headers through, that client can grant itself any scope. The authorizer refuses to start until the config acknowledges this with `trusted: true`.

```yaml
authorizer:
  type: header
  config: ./examples/config/global-query-header-authorizer-example-config.yaml
```

Config file (see the [example](../../examples/config/global-query-header-authorizer-example-config.yaml)):

| Key                | Default                   | Meaning                                                                                       |
|--------------------|---------------------------|-----------------------------------------------------------------------------------------------|
| `trusted`          | none, required            | Must be `true`: acknowledges that clients cannot set the scope header. Anything else, or no config file at all, fails startup |
| `scope_header`     | `X-GoProbe-Allowed-Hosts` | Header carrying the comma-separated **host IDs** of the scope                                 |
| `principal_header` | `X-GoProbe-Principal`     | Header carrying the **principal**, audit only. `unknown` when absent, empty or sent more than once |

Rules:

- The scope header holds a comma-separated list of host IDs. Each is trimmed, duplicates are dropped, and host IDs are matched exactly. There is no wildcard: a literal `*` is an ordinary host ID that matches nothing.
- A missing scope header, an empty one (also one holding only whitespace and commas), or one arriving **more than once** is answered with `403`. The repetition rule catches a gateway that appends a second header line instead of replacing the client's; it is a safety net, not a substitute for the gateway replacing the header. It cannot catch an intermediate proxy that folds repeated headers into one comma-joined value (RFC 9110 allows combining list-valued headers, and some gateways do it): a client-sent value folded into the gateway's arrives as a single header and passes. Hence the gateway must **replace**, never append, and no proxy between gateway and `global-query` may fold headers. The authorizer never answers `401`: it does not authenticate.
- The principal is the value of the principal header. It is reported as `unknown` when the header is absent, empty or arrives **more than once** (with an appending gateway the first value would be the client's, and the audit trail must not record it), capped at 256 runes before it is stored and logged, and never influences the decision: a repeated principal header does not reject the request.
- Unknown keys in the config file fail startup with an error naming the key, so a misspelled header name cannot silently fall back to the default. The file must hold a single YAML document: a second one (after a `---` separator) fails startup too, since keys in it would be ignored and leave the authorizer reading a default header the gateway does not replace.

**Scope size bound.** The scope travels in a request header, and proxies commonly cap request headers at around 8 KB. A scope of more than a few hundred host IDs will not fit; that is the point where a provider-backed authorizer, which looks the scope up by principal, is needed instead.

## Running Global Queries

A global query is run analogously to the way you would query local data via the `goProbe` API: via the `/_query` endpoint.

The parameters which need to be provided are the JSON-serialized [`query.Args`](../../pkg/query/args.go). The main difference to calling the endpoint directly on the `goProbe` API is that the `query_hosts` parameter needs to be explicitly provided in order to tell the query server which host(s) should be queried.

## API Documentation

The global-query API is described by an OpenAPI 3.0.3 specification generated from the registered routes. A running server serves it at `/openapi.yaml` (also `/openapi.json`) with interactive documentation at `/docs`. To write it to a file without starting the server:

```sh
go run ./cmd/global-query --config examples/config/global-query-example-config.yaml server --openapi.spec-outfile global-query_openapi.yaml
```

The output is a single file. Every GitHub release also attaches it as `global-query_<version>_openapi.yaml`.
