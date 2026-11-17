# gpctl

> CLI tool to query and modify goProbe's internal state

## Invocation

```sh
./gpctl -s unix:/var/run/goprobe status eth0 eth1
```

This will produce the capture statistics (processed packets, drops, active capture, etc.) for interfaces eth0 and eth1.

### Reloading goProbe's Configuration

To make goProbe re-read its own on-disk configuration file and apply the `interfaces` section, run

```sh
./gpctl -s unix:/var/run/goprobe config -r
```

To push the `interfaces` section of a different file without touching goProbe's on-disk configuration, use `config -f /path/to/goprobe.yaml`. Add `--silent` to suppress the per-interface change listing.

### Flags and subcommands

| Flag / command | Meaning |
|---|---|
| `-s, --server.addr` | goProbe API address (`host:port` or `unix:/path`) |
| `-t, --timeout` | request timeout (default `10s`) |
| `--config` | gpctl configuration file |
| `status [IFACES] -v` | capture statistics; `-v` adds packet-parsing error counters |
| `config -r \| -f FILE [--silent]` | reload / apply interface configuration |
| `version` | print version |

## Configuration

To avoid having to specify goProbe's API server address with every call, it is recommended to provide a minimal configuration
file guiding API query behavior and creating an alias:

```sh
alias gpctl="./gpctl --config /path/to/gpctl.yaml"
```

Refer to [gpctl-example-config.yaml](../../examples/config/gpctl-example-config.yaml) for configuration options.
