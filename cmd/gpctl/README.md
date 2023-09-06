# gpctl

> CLI tool to query and modify goProbe's internal state

## Invocation

```sh
./gpctl -s unix:/var/run/goprobe status eth0 eth1
```

This will produce the capture statistics (processed packets, drops, active capture, etc.) for interfaces eth0 and eth1.

### Reloading goProbe's Configuration

To force a configuration reload of goProbe's interface configuration, point to its configuration file and run

```sh
./gpctl -s unix:/var/run/goprobe config -f /path/to/goprobe.yaml
```

## Configuration

To avoid having to specify goProbe's API server address with every call, it is recommended to provide a minimal configuration
file guiding API query behavior and creating an alias:

```sh
alias gpctl="./gpctl --config /path/to/gpctl.yaml"
```

Refer to [gpctl-example-config.yaml](../../examples/config/gpctl-example-config.yaml) for configuration options.
