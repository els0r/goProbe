# gpdb

> CLI tool to maintain on-disk goProbe databases

## Invocation

```sh
./gpdb merge /path/to/source-db /path/to/destination-db
```

This merges source goDB data into the destination database.

- Complete day folders are copied directly when safe.
- Partial day folders are rebuilt block-by-block so metadata is re-encoded on write.
- If both sides contain the same block timestamp, destination data wins by default.

### Common options

```sh
./gpdb merge /path/to/source-db /path/to/destination-db \
  --iface eth0 --iface eth1 \
  --dry-run
```

Use `--overwrite` to prefer source data on conflicts and to replace complete destination days with complete source days.

## Configuration

`gpdb` currently operates directly on local database paths and does not require API server configuration.

Refer to [goDB Database Format](../../pkg/goDB/database_format.md) for layout and metadata details.
