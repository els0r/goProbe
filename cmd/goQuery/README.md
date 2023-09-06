# goQuery

> CLI tool for high-performance querying of flow data acquired by goProbe

The tool `goQuery` is responsible for querying and displaying the flow data captured by `goProbe`. It can be thought of as the go-to tool for a human to analyze the captured traffic.

## Invocation

```sh
./goQuery -d /path/to/godb -i eth0 -c "dport=443" -n 10 sip,dip

                                   packets   packets             bytes      bytes
              sip            dip        in       out      %         in        out      %
      10.236.2.56  10.236.130.23  594.33 k  847.92 k  54.52   70.23 MB  793.10 MB  52.79
      10.236.2.56  10.236.146.23  480.85 k  722.45 k  45.48   59.90 MB  712.25 MB  47.21
  215.165.238.169    10.236.2.61    8.00      0.00     0.00  488.00  B    0.00  B   0.00
  213.156.238.169    10.236.2.56    6.00      0.00     0.00  396.00  B    0.00  B   0.00
  213.156.238.168    10.236.2.56    6.00      0.00     0.00  396.00  B    0.00  B   0.00
  213.156.238.168    10.236.2.61    4.00      0.00     0.00  244.00  B    0.00  B   0.00

                                    1.08 M    1.57 M         130.13 MB    1.47 GB

          Totals:                             2.65 M                      1.60 GB

Timespan / Interface : [2023-08-18 01:55:00, 2023-08-26 03:45:00] (8d1h50m0s) / eth0
Sorted by            : accumulated data volume (sent and received)
Query stats          : displayed top 6 hits out of 6 in 37ms
Conditions:          : dport = 443

```

The list of available options is rich, so it's best to familiarize oneself with them via

```sh
./goQuery --help
```

### Local goDB

The standard way to query flow data is via a flow database (goDB) stored on the same host as where `goQuery` is invoked. The parameter `--database|-d` will instruct `goQuery` to load and aggregate flow data from a local directory.

This is the default case.

### Global Query Server

If command line parameter `--query.server.addr` is provided and a list of hosts to query via `-q|query.hosts-resolution`, the query will be sent to a [global-query](../global-query/) server instead.

This scenario requires the query server to be reachable at `--query.server.addr` and presumes that the query server in turn is able to reach the `goProbe` API on the list of hosts provided in the query.

If this mode is used, the attribute `hostname` will always be provided in the output of `goQuery`.

### Stored queries

Query arguments are JSON serializable and `goQuery` offers the ability to load them from disk and run a query based on the stored args.

This has the advantage that it allows you to configure scheduled tasks without having to change the flags of `goQuery` and hence not the programs or scripts calling it.

To execute a stored query, run

```sh
./goQuery --stored-query /path/to/args.json
```

The args file can look as follows:

```json
{
  "query": "sip,dip,proto",
  "ifaces": "eth0,eth1",
  "condition": "dport=443",
  "in": true,
  "out": true,
  "sum": false,
  "first": "01.03.2019 00:00",
  "last": "31.03.2023 23:59",
  "format": "json",
  "sort_by": "bytes",
  "num_results": 10,
  "sort_ascending": false,
  "dns_resolution": {
    "enabled": true,
    "timeout": "1s",
    "max_rows": 25
  },
  "max_mem_pct": 25,
  "caller": "batch-job-XYZ"
}
```

## Configuration

While the query parameters are supposed to be provided on invocation, base parameters such as the DB path or the query server address can be provided in configuration.

To avoid having to specify them with every call, it is recommended to provide a minimal configuration
file guiding query behavior and creating an alias:

```sh
alias goquery="./goQuery --config /path/to/goquery.yaml"
```

Refer to [goquery-example-config.yaml](../../examples/config/goquery-example-config.yaml) for configuration options. If both `db.path` and `query.server.addr` are specified, the local query mode via DB takes precedence.

## Retention / Information Lifecycle Management

Due to the compact size of the flow data stored in `goDB` and cheap availability of disk space, there is no real need to implement a retention policy as part of the goProbe software suite.

*It is recommended to store flow data indefinitely.*

For high-throughput systems with limited disk space, it may still be beneficial to rotate out database information older than X days. Consider installing a cronjob on the target system:

```sh
# Run goprobe database cleanup (retention time 180 days)
3 3 * * *  root RETENTION_DAYS=180; DB_PATH=/path/to/godb/; test -e "$DB_PATH" && find "${DB_PATH}" -links 2 -type d -mtime +"${RETENTION_DAYS}" -exec rm -rf {} \;
```
