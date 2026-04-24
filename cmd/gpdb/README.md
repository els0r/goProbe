# gpdb

> CLI tool to maintain on-disk goProbe databases

## Subcommands

### merge

```sh
./gpdb merge /path/to/source-db /path/to/destination-db
```

This merges source goDB data into the destination database.

- Complete day folders are copied directly when safe.
- Partial day folders are rebuilt block-by-block so metadata is re-encoded on write.
- If both sides contain the same block timestamp, destination data wins by default.

Common options:

```sh
./gpdb merge /path/to/source-db /path/to/destination-db \
  --iface eth0 --iface eth1 \
  --dry-run
```

Use `--overwrite` to prefer source data on conflicts and to replace complete destination days with complete source days.

### import

```sh
./gpdb import /path/to/input.csv /path/to/destination-db
```

This imports flow rows from CSV into goDB.

- If `--schema` is omitted, the first CSV row is interpreted as schema/header.
- Schema must include `time` and either `iface` or `--iface` must be provided.
- Input rows must be ordered by non-decreasing `time`.

Common options:

```sh
./gpdb import /path/to/input.csv /path/to/destination-db \
  --schema "time,sip,dip,dport,proto,packets received,packets sent,data vol. received,data vol. sent" \
  --iface eth0 \
  --encoder lz4 \
  --max-rows 100000
```

`--permissions` accepts numeric modes like `0644`.

## Configuration

`gpdb` currently operates directly on local database paths and does not require API server configuration.

Refer to [goDB Database Format](../../pkg/goDB/database_format.md) for layout and metadata details.
