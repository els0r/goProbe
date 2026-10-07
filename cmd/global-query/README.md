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

No in-tree authorizer ships yet. Out-of-tree authorizers register through the [contrib mechanism](../../plugins/contrib/README.md), which also documents the contract for plugin authors.

## Running Global Queries

A global query is run analogously to the way you would query local data via the `goProbe` API: via the `/_query` endpoint.

The parameters which need to be provided are the JSON-serialized [`query.Args`](../../pkg/query/args.go). The main difference to calling the endpoint directly on the `goProbe` API is that the `hosts_query` parameter needs to be explicitly provided in order to tell the query server which host(s) should be queried.

## API Documentation

The global-query API is laid out in the [OpenAPI 3.0 Specification](../../pkg/api/globalquery/spec/openapi.yaml).

**Note**: some tools only accept a single OpenAPI file. To merge the specification into one output file, use [`swagger-cli`](https://www.npmjs.com/package/swagger-cli):

```sh
swagger-cli bundle ../../pkg/api/globalquery/spec/openapi.yaml --outfile _build/openapi.yaml --type yaml
```
