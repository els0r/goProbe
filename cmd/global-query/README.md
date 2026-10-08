# global-query

> Query server able to aggregate query results across a fleet of hosts

The tool `global-query` is the central entrypoint for running queries across a fleet of hosts. If supplied with configuration on how to reach sensors running `goProbe`, it will be able to run queries on said hosts.

It's the API pendant to `goQuery`. While goQuery is usually used for running queries against a local DB on the same system, on which capturing is performed, `global-query` will aggregate results across a set of hosts.

The query server is meant to be deployed centrally (i.e. in a kubernetes cluster) and should serve as the backend for a front-end displaying results. `goQuery` itself can take the role of that front end and display aggregated results.

## Getting Started

```sh
go run main.go --config global-query-config.yaml server
```

## Deployment

A `Dockerfile` will follow in future releases. The tool is written in pure `go` and does not have any low-level dependencies (in contrast to `goProbe`/`goQuery`).

## Distributed Query Runner

Aggregation of results is purely based on the [`results.Results`](../../pkg/results/result.go) data structure.

The query server is set up in a way that different query runners can be used to fetch the results from the sensors. The default distributed query runner is the [`APIClientQuerier`](./pkg/distributed/querier.go), which uses the [`goProbe` Client](../../pkg/api/goprobe/client/) to fetch results from a sensor.

### API Client Querier Configuration

Each distributed query runner will rely on specific configuration. In the case of the API client querier, the `global-query` server needs knowledge on how to reach a given host that was specified in the hosts list.

An example configuration for the API Client Querier is available under [global-query-api-client-querier-example-config.yaml](../../examples/config/global-query-api-client-querier-example-config.yaml).

### Custom Query Runners

In future releases, the plugin system will be built out so that other queriers can be used. There are two requirements:

* the `query.Runner` interface is implemented
* the use case specific configuration is supplied to `global-query` upon initialization

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

**Sensors remain unauthenticated.** Only `global-query` is scoped. A `goProbe` sensor still answers anyone who can reach its API, so the path between `global-query` and the sensors must stay private: it must not be reachable by the callers of `global-query`.

One authorizer ships in-tree, `header`, described below. Out-of-tree authorizers register through the [contrib mechanism](../../plugins/contrib/README.md), which also documents the contract for plugin authors.

### The `header` authorizer

The `header` authorizer reads the **scope** from a request header set by a trusted gateway in front of `global-query`, and optionally the **principal** from a second header. It lets an operator enable scoping without writing code.

> **Warning: the `header` authorizer is exactly as safe as the network path in front of it.** It trusts the scope header unconditionally. The gateway must strip or replace the scope and principal headers on **every** path a client can reach, and must replace rather than append. If a client can reach `global-query` directly, or through any path that passes its headers through, that client can grant itself any scope. The authorizer refuses to start until the config acknowledges this with `trusted: true`.

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
| `principal_header` | `X-GoProbe-Principal`     | Header carrying the **principal**, audit only                                                 |

Rules:

- The scope header holds a comma-separated list of host IDs. Each is trimmed, duplicates are dropped, and host IDs are matched exactly. There is no wildcard: a literal `*` is an ordinary host ID that matches nothing.
- A missing scope header, an empty one (also one holding only whitespace and commas), or one present **more than once** is answered with `403`. The repetition rule means a gateway that appends instead of replacing cannot be bypassed by a client sending its own value first. The authorizer never answers `401`: it does not authenticate.
- The principal is the first value of the principal header. It is reported as `unknown` when the header is absent or empty, capped at 256 runes before it is stored and logged, and never influences the decision.

**Scope size bound.** The scope travels in a request header, and proxies commonly cap request headers at around 8 KB. A scope of more than a few hundred host IDs will not fit; that is the point where a provider-backed authorizer, which looks the scope up by principal, is needed instead.

## Running Global Queries

A global query is run analogously to the way you would query local data via the `goProbe` API: via the `/_query` endpoint.

The parameters which need to be provided are the JSON-serialized [`query.Args`](../../pkg/query/args.go). The main difference to calling the endpoint directly on the `goProbe` API is that the `hosts_query` parameter needs to be explicitly provided in order to tell the query server which host(s) should be queried.

## API Documentation

The global-query API is laid out in the [OpenAPI 3.0 Specification](../../pkg/api/globalquery/spec/openapi.yaml).

**Note**: some tools only accept a single OpenAPI file. To merge the specification into one output file, use [`swagger-cli`](https://www.npmjs.com/package/swagger-cli):

```sh
swagger-cli bundle ../../pkg/api/globalquery/spec/openapi.yaml --outfile _build/openapi.yaml --type yaml
```
