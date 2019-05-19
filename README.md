goProbe
[![GoDoc](https://godoc.org/github.com/els0r/goProbe?status.svg)](https://godoc.org/github.com/els0r/goProbe/) [![Go Report Card](https://goreportcard.com/badge/github.com/els0r/goProbe)](https://goreportcard.com/report/github.com/els0r/goProbe)[![Build Status](https://cloud.drone.io/api/badges/els0r/goProbe/status.svg)](https://cloud.drone.io/els0r/goProbe)
===========

This package comprises:

* goProbe   - A lightweight, concurrent, network packet aggregator
* goDB      - A small, high-performance, columnar database (pkg)
* goQuery   - A CLI tool using the query front-end to read out data acquired by goProbe and stored in goDB
* goConvert - Helper binary to convert goProbe-flow data stored in `csv` files

As the name suggests, all components are written in [Go](https://golang.org/).

Introduction
------------

Today, targeted analyses of network traffic patterns have become increasingly difficult due to the sheer amount of traffic encountered. To enable them, traffic needs to be captured and examined and broken down to key descriptors which yield a condensed explanation of the underlying data.

The [NetFlow](http://www.ietf.org/rfc/rfc3954.txt) standard was introduced to address this reduction. It uses the concept of flows, which combine packets based on a set of shared packet attributes. NetFlow information is usually captured on one device and collected in a central database on another device. Several software probes are available, implementing NetFlow exporters and collectors.

goProbe deviates from traditional NetFlow as flow capturing and collection is run on the same device and the flow fields reduced. It was designed as a lightweight, standalone system, providing both optimized packet capture and a storage backend tailored to the flow data in order to provide lightning-fast analysis queries.

Quick Start
------------

```
go get github.com/els0r/goProbe/...
```

See the installation section for more details.

The package itself now requires a fully set up Go environment. Running install/build will suffice to build the binaries goProbe, goQuery and goConvert.

```
go install -i github.com/els0r/goProbe/...
```

Alternatively, you can use `go generate` from `gen.go` to install all binaries (and run tests and linters):
```
go generate
```

The addon folder provides a Makefile for building the software suite. To use it, run

```
cd addon
make all
```

goProbe
-------------------------
`goProbe` captures packets using [libpcap](http://www.tcpdump.org/) and [gopacket](https://code.google.com/p/gopacket/) and extracts several attributes which are used to classify the packet in a flow-like data structure:

* Source and Destination IP
* IP Protocol
* Destination Port (if available)

Available flow counters are:

* Bytes sent and received
* Packet sent and received

In summary: *a goProbe-flow is not a NetFlow-flow*.

The flow data is written out to a custom colum store called `goDB`, which was specifically designed to accomodate goProbe's data. Each of the above attributes is stored in a column file

### Usage

Capturing is performed concurrently by goProbe on multiple interfaces. goProbe is started as follows (either as `root` or as non-root with capability `CAP_NET_RAW`):

```
/opt/ntm/goProbe/bin/goProbe -config <path to configuration file>
```
The capturing probe can be run as a daemon via

```
/etc/init.d/goprobe.init {start|stop|status|restart|reload|force-reload}
```

### Configuration

You must configure goProbe. By default, the relevant configuration file resides in
`/opt/ntm/goProbe/etc/goprobe.conf`. The config covers three aspects of goProbe: capturing, logging and the API.

An example configuration file is created during installation at `/opt/ntm/goProbe/etc/goprobe.conf.example`.

#### DB location

The location of the goDB database to which flows are written is configured with `dbpath` , e.g. `"db_path" : "/path/to/database"`

#### Interface

The interface configuration is stored as JSON and looks as follows:
```
"interfaces" : {                           // configure each interface we want to listen on
  "eth0" : {
    "bpf_filter" : "not arp and not icmp", // bpf filter string like for tcpdump
    "buf_size" : 2097152,                  // pcap buffer size
    "promisc" : false                      // enable promiscuous mode
  },
  "eth1" : {
    "bpf_filter" : "not arp and not icmp",
    "buf_size" : 1048576,
    "promisc" : true
  }
}
```

Changes to the interface configuration can be _live reloaded_.

#### Logging

goProbe has flexible logging capabilities. It uses the `Logger` interface from third-party package [log](https://github.com/els0r/log), which is compatible with most third-party logging frameworks. Hence, other loggers can be injected into goProbe.

The default configuration has goProbe log to syslog with level "info". The config blocks looks as follows:
```
"logging" : {
    "destination" : "console", // will write log messages to stdout/stderr
    "level" : "debug"          // more verbose logging
}
```

Changes to the logging configuration require a _restart_ of goProbe.

#### API

By default, goProbe spawns a command-and-control HTTP API server. For more information on the capabilities of the API, see [API documentation](./pkg/api/README.md).

The API itself is configured via the following parameters:
```
"api" : {
    "host" : "localhost",
    "port" : "6060",          // port to bind to
    "request_logging" : true, // log API usage
    "request_timeout" : 60,   // maximum request duration (in seconds)
    "keys" : [                // auth via pre-shared keys (don't use below in production!)
        "da53ae3fb482db63d9606a9324a694bf51f7ad47623c04ab7b97a811f2a78e05",
        "9e3b84ae1437a73154ac5c48a37d5085a3f6e68621b56b626f81620de271a2f6"
    ]
}
```

Changes to the logging configuration mostly require a _restart_ of goProbe (for more info see below).

#### Service discovery and auto-registration

If goProbe should advertise how it can be reached, auto-registration can be enabled. This will make goProbe periodically attempt to register its API details at an [ntm-discovery-service](./addon/ntm-discovery-service) endpoint.

To enable this, enrich the API configuration:
```
"api" : {
    ...,
    "service_discovery" : {
        "endpoint" : "https://goprobe.mydomain.com/server1",    // the endpoint does not have to coincide with the API settings in order to support setups behind a web-application firewall
        "probe_identifier" : "dc-server1",                      // a unique identifier for the host on which goProbe runs
        "skip_verify" : false,                                  // in case self-signed certificates are used by the registry ("true" disables SSL cert verification)
        "registry" : "https://goprobe-discovery.mydomain.com"   // location of the discovery service
    }
}
```

Addition of a service discovery configuration require a _restart_ of goProbe. Changes (modifications and deletion of config) can be _live reloaded_.

goDB
--------------------------
The flow records are stored block-wise on a five minute basis in their respective attribute files. The database is partitioned on a per day basis, which means that for each day, a new folder is created which holds the attribute files for all flow records written throughout the day.

Blocks are compressed using [lz4](https://code.google.com/p/lz4/) compression, which was chosen to enable both swift decompression and good data compression ratios.

`goDB` is a package which can be imported by other `go` applications.

goQuery
--------------------------

`goQuery` is the query front which is used to access and aggregate the flow information stored in the database. The following query types are supported:

* Top talkers: show data traffic volume of all unique IP pairs
* Top Applications (port/protocol): traffic volume of all unique destination port-transport protocol pairs, e.g., 443/TCP

### Usage

For a comprehensive help on how to use goQuery type `/opt/ntm/goProbe/bin/goQuery -h` or `/opt/ntm/goProbe/bin/goQuery help`.

### Example Output

```
# goQuery -i eth0 -c 'dport = 443' -n 10 sip,dip

                                   packets   packets             bytes      bytes
             sip             dip        in       out      %         in        out      %
  125.167.76.152  237.147.182.13  308.75 k  576.81 k  66.95   17.71 MB  805.53 MB  64.33
  121.18.119.116  125.167.76.152  149.81 k   24.00 k  13.14  198.06 MB    9.64 MB  16.23
  125.167.76.152  121.18.119.116  116.20 k   27.16 k  10.84  151.00 MB   14.18 MB  12.91
  125.167.76.152  121.18.250.176   15.29 k   22.14 k   2.83   21.22 MB   18.26 MB   3.09
  125.167.76.152   51.143.39.255    3.77 k    2.51 k   0.47    5.55 MB  271.98 kB   0.45
  125.167.76.152   55.135.93.254    1.23 k    1.84 k   0.23    3.06 MB  197.34 kB   0.25
  125.167.76.152  233.41.242.235  813.00      1.15 k   0.15    2.25 MB  143.61 kB   0.19
  125.167.76.152  190.14.221.249  503.00    764.00     0.10    1.55 MB  120.58 kB   0.13
  125.167.76.152   11.26.172.240    2.13 k    1.52 k   0.28    1.40 MB  232.91 kB   0.13
  125.167.76.152  55.135.212.216  571.00    806.00     0.10    1.41 MB  133.44 kB   0.12
                                       ...       ...               ...        ...
                                  630.68 k  692.11 k         424.39 MB  855.27 MB

         Totals:                              1.32 M                      1.25 GB

Timespan / Interface : [2016-02-25 19:29:35, 2016-02-26 07:44:35] / eth0
Sorted by            : accumulated data volume (sent and received)
Query stats          : 268.00   hits in 17ms
Conditions:          : dport = 443
```

### Converting data

If you use `goConvert`, you need to make sure that the data which you are importing is _temporally ordered_ and provides a column which stores UNIX timestamps. An example `csv` file may look as follows:

```
# HEADER: bytes_rcvd,bytes_sent,dip,dport,packets_rcvd,packets_sent,proto,sip,tstamp
...
40,72,172.23.34.171,8080,1,1,6,10.11.72.28,1392997558
40,72,172.23.34.171,49362,1,1,6,10.11.72.28,1392999058
...
```
You _must_ abide by this structure, otherwise the conversion will fail.

Query interface
--------------------------

Under the hood, `goQuery` uses the [query](pkg/query) API to access the goDB and run queries on it. To prepare and run a query, the `query.Args` type has to be populated.

It is recommended to use `query.NewArgs()` instead of building up the struct from scratch. This will ensure that sensible defaults are set already.

For an example how to use it in your code, please refer to the [query API README](pkg/query/README.md).

### Stored queries

Query arguments are JSON serializable and `goQuery` offers the ability to load them from disk and run a query based on the stored args.

This has the advantage that it allows you to configure scheduled tasks without having to change the flags of `goQuery` and hence not the programs or scripts calling it.

To run a stored query, run
```
goQuery --stored-query /path/to/args.json
```

The args file can look as follows:
```
{
  "Query": "sip,dip,proto",
  "Output": "/tmp/query.output",  // where to route the output (stdout is default)
  "Ifaces": "eth0,eth1",
  "Condition": "dport eq 443",
  "In": true,
  "Out": true,
  "Sum": false,
  "First": "01.03.2019 00:00",
  "Last": "31.03.2019 23:59",
  "Format": "json",
  "SortBy": "bytes",
  "NumResults": 10,               // only show the top 10 rows
  "SortAscending": false,         // reverse sort order
  "Resolve": false,               // attempt to reverse lookup IP addresses
  "ResolveTimeout": 1,
  "ResolveRows": 25,
  "MaxMemPct": 25,                // use at most 25% of the available memory for the query
  "Caller": "batch-job-XYZ"       // identifier who called the query
}
```

Installation
------------

*Note*: the default directory for `goProbe` is `/opt/ntm/goProbe`. If you wish to change this, change the `PREFIX` variable in the `Makefile` to a destination of your choosing.

Before running the installer, make sure that you have the following dependencies installed:
* golang
* socat (for init script usage only)
* rsync (for deploy target only)

The package is designed to only require a fully set up `go` environment. To build everything via Makefile, run

```
make all
```

Otherwise, running install/build will suffice

```
go install -i github.com/els0r/goProbe/cmd/goProbe
go install -i github.com/els0r/goProbe/cmd/goQuery
go install -i github.com/els0r/goProbe/cmd/goConvert
```

Additional Makefile targets for deployment are:
* `make deploy`: syncs the binary tree to the root directory. *Note:* this is only a good idea if you want to run goProbe on the system where you compiled it.
* `make package`: creates a tarball for deployment on another system.

The binary will reside in the directory specified in the above command.

### Bash autocompletion

goQuery has extensive support for bash autocompletion. To enable autocompletion,
you need to tell bash that it should use the `goquery_completion` program for
completing `goquery` commands.
How to do this depends on your distribution.
On Debian derivatives, we suggest creating a file `goquery` in `/etc/bash_completion.d` with the following contents:
```
_goquery() {
    case "$3" in
        -d) # the -d flag specifies the database directory.
            # we rely on bash's builtin directory completion.
            COMPREPLY=( $( compgen -d -- "$2" ) )
        ;;

        *)
            if [ -x /opt/ntm/goProbe/shared/goquery_completion ]; then
                mapfile -t COMPREPLY < <( /opt/ntm/goProbe/shared/goquery_completion bash "${COMP_POINT}" "${COMP_LINE}" )
            fi
        ;;
    esac
}
```

### Supported Operating Systems

goProbe is currently set up to run on Linux based systems and Mac OS X. Tested versions include (but are most likely not limited to):

* Ubuntu 14.04/15.04
* Debian 7/8/9
* Fedora 28
* Mac OS X 10.14.3

Authors & Contributors
----------------------

* Lennart Elsen
* Fabian Kohn
* Lorenz Breidenbach

This software was developed at [Open Systems AG](https://www.open.ch/) in close collaboration with the [Distributed Computing Group](http://www.disco.ethz.ch/) at the [Swiss Federal Institute of Technology](https://www.ethz.ch/en.html).

Bug Reports
-----------

Please use the [issue tracker](https://github.com/els0r/goProbe/issues) for bugs and feature requests.

License
-------
See the LICENSE file for usage conditions.
