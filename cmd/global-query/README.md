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

## Running Global Queries

A global query is run analogously to the way you would query local data via the `goProbe` API: via the `/_query` endpoint.

The parameters which need to be provided are the JSON-serialized [`query.Args`](../../pkg/query/args.go). The main difference to calling the endpoint directly on the `goProbe` API is that the `hosts_query` parameter needs to be explicitly provided in order to tell the query server which host(s) should be queried.

## API Documentation

The global-query API is laid out in the [OpenAPI 3.0 Specification](../../pkg/api/globalquery/spec/openapi.yaml).

**Note**: some tools only accept a single OpenAPI file. To merge the specification into one output file, use [`swagger-cli`](https://www.npmjs.com/package/swagger-cli):

```sh
swagger-cli bundle ../../pkg/api/globalquery/spec/openapi.yaml --outfile _build/openapi.yaml --type yaml
```
