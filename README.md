goProbe
===========

This package comprises:

* goProbe   - A lightweight, concurrent, network packet aggregator
* goDB      - A small, high-performance, columnar database
* goQuery   - Query front-end used to read out data acquired by goProbe and stored by goDB
* goConvert - Helper binary to convert goProbe-flow data stored in `csv` files

As the name suggests, all components are written in Google [go](https://golang.org/).

Introduction
------------

Today, targeted analyses of network traffic patterns have become increasingly difficult due to the sheer amount of traffic encountered. To enable them, traffic needs to be captured and examined and broken down to key descriptors which yield a condensed explanation of the underlying data.

The [NetFlow](http://www.ietf.org/rfc/rfc3954.txt) standard was introduced to address this reduction. It uses the concept of flows, which combine packets based on a set of shared packet attributes. NetFlow information is usually captured on one device and collected in a central database on another device. Several software probes are available, implementing NetFlow exporters and collectors.

goProbe deviates from traditional NetFlow as flow capturing and collection is run on the same device and the flow fields reduced. It was designed as a lightweight, standalone system, providing both optimized packet capture and a storage backend tailored to the flow data.

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
`/opt/ntm/goProbe/etc/goprobe.conf`.
The configuration is stored as JSON and looks like this:
```
{
  "db_path" : "/path/to/database",
  "interfaces" : { // configure each interface we want to listen on
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
}
```

An example configuration file is created during installation at `/opt/ntm/goProbe/etc/goprobe.conf.example`.

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

For a comprehensive help on how to use goQuery type `/opt/ntm/goProbe/bin/goQuery -h`

Example Output
------

```
# goquery -i eth0 -c 'dport = 443' -n 10 sip,dip

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

Logging Facilities
------------------

Both goProbe and goDB write to the Syslog facility. However, the log output is passed to syslog via UDP packets to destination port 514. You will have to make sure that your syslog daemon supports logging via UDP. On most platforms uncommenting the following in `/etc/rsyslog.conf` should suffice:

```
$ModLoad imudp
$UDPServerRun 514
```

Changes should take effect after rebooting the machine.

Installation
------------

*Note*: the default directory for `goProbe` is `/opt/ntm/goProbe`. If you wish to change this, change the `PREFIX` variable in the `Makefile` to a destination of your choosing.


Before running the installer, make sure that you have the following dependencies installed:
* yacc
* bison
* curl
* build-essential
* flex
* socat
* rsync

The package itself was designed to work out of the box. Thus, you do not even need the `go` environment. All of the dependencies are downloaded during package configuration. To install the package, go to the directory into which you cloned this repository and run the following commands:

```
sudo apt-get install yacc bison curl build-essential flex socat rsync
make all
```

Additional Makefile targets for deployment are:
* `make deploy`: syncs the binary tree to the root directory. *Note:* this is only a good idea if you want to run goProbe on the system where you compiled it.
* `make package`: creates a tarball for deployment on another system.

By default, `goConvert` is not compiled. If you wish to do so, add the following line to the `install` target in the Makefile:

```
go build -a -o goConvert $(PWD)/addon/gocode/src/OSAG/convert/DBConvert.go
```
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

goProbe is currently set up to run on Linux based systems. Tested versions include:

* Ubuntu 14.04/15.04
* Debian 7/8/9
* Fedora 28

Authors & Contributors
----------------------

* Lennart Elsen, Open Systems AG
* Fabian Kohn, Open Systems AG
* Lorenz Breidenbach, Open Systems AG

This software was developed at [Open Systems AG](https://www.open.ch/) in close collaboration with the [Distributed Computing Group](http://www.disco.ethz.ch/) at the [Swiss Federal Institute of Technology](https://www.ethz.ch/en.html).

License
-------
See the LICENSE file for usage conditions.
