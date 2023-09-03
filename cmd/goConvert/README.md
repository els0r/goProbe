# goConvert

> Ingest flow data from CSV files and convert it to GPFiles for goDBv

## Quick Start

How to run

```sh
go run goConvert.go --help
```

## Ingesting Flow Data

You need to make sure that the data which you are importing is _ordered by time_ and provides a column which stores UNIX timestamps. An example `csv` file may look as follows:

```sh
# HEADER: bytes_rcvd,bytes_sent,dip,dport,packets_rcvd,packets_sent,proto,sip,tstamp
...
40,72,172.23.34.171,8080,1,1,6,10.11.72.28,1392997558
40,72,172.23.34.171,49362,1,1,6,10.11.72.28,1392999058
...
```

You _must_ abide by this structure, otherwise the conversion will fail.
