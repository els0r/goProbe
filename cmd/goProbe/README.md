# goProbe

> High-performance network packet metadata capture and storage of flows

The tool `goProbe` is responsible for capturing packet metadata _off the wire_. Under the hood, it makes use of [slimcap](https://github.com/fako1024/slimcap) to extract the following attributes which are used to classify the packet in a flow-like data structure:

* `sip`: source IP
* `dip`: destination IP
* `dport`: destination port (if available)
* `proto`: IP protocol

Available flow counters are

* `bytes_sent`: bytes sent
* `bytes_rcvd`: bytes received
* `pkts_sent`: packets sent
* `pkts_rcvd`: packets received

Note: _a goProbe-flow is hence not a NetFlow-flow_. Nonetheless, the limited metadata collected in a goProbe-flow has helped resolved numerous network incidents and mis-configurations for almost a decade at Open Systems AG and half a decade at nect.

## Invocation

To start capturing, run

```sh
./goProbe -config goprobe.yaml
```

The tool is meant to run as a service/daemon by means of init scripts or systems such as `systemctl`. Examples for such intergrations can be found inside the [examples/config](../../examples/config) folder.

## Configuration

Refer to [goprobe-example-config.yaml](../../examples/config/goprobe-example-config.yaml) for configuration options.

The configuration can be provided as YAML or as JSON.

### Live Config

The `interfaces` section of the configuration file is watched by goProbe and reloaded periodically. This is in order to reflect changes to individual interfaces without having to restart capturing. This ensures that only the affected interfaces have a short downtime while capturing resumes for all other interfaces.

All other changes to the configuration _require a restart of goProbe_.

## API

By default, goProbe spawns a command-and-control HTTP API server, to provide access to its internal state as well as a query API to to query data from the goDB database to which it writes.

The API is able to bind on UNIX sockets.

### Documentation

The goProbe API is laid out in the [OpenAPI 3.0 Specification](../../pkg/api/goprobe/spec/openapi.yaml).

**Note**: some tools only accept a single OpenAPI file. To merge the specification into one output file, use [`swagger-cli`](https://www.npmjs.com/package/swagger-cli):

```sh
swagger-cli bundle ../../pkg/api/goprobe/spec/openapi.yaml --outfile _build/openapi.yaml --type yaml
```

### Using `gpctl`

The tool [gpctl](../gpctl/) was specifically designed to cover the more common control API calls to inspect `goProbe`'s internal state.

Example:

```sh
gpctl --server.addr unix:/var/run/goprobe status eth0 eth1
```

### Client

There is a [client](../../pkg/api/goprobe/client/) package available that allows to make calls to the API programmatically and retrieve data structures used by `goProbe`.

Both `gpctl` and `global-query` use it internally.
