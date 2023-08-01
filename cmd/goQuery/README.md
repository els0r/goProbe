# goQuery

> CLI tool for high-performance querying of goDB flow data acquired by goProbe

## Invocation

### Local goDB

### Global Query Server

## Configuration

## Retention / Information Lifecycle Management

Due to the compact size of the flow data stored in `goDB` and cheap availability of disk space, there is no real need to implement a retention policy as part of the goProbe software suite.

It is recommended to store flow data indefinitely.

For high-throughput systems with limited disk space, it may still be beneficial to rotate out database information older than X days. Consider installing a cronjob on the target system:

```sh
# Run goprobe database cleanup (retention time 180 days)
3 3 * * *  root RETENTION_DAYS=180; DB_PATH=/path/to/godb/; test -e "$DB_PATH" && find "${DB_PATH}" -type d -mtime +"${RETENTION_DAYS}" -exec rm -rf {}
```
