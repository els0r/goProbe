# Feasibility Study: Historic Interface IP Storage for goProbe

## 1. Background and Motivation

goProbe captures network flow data per interface and stores it in a columnar binary
format (.gpf files) organized by interface and time. Two query tools — `goquery`
(local) and `global-query` (distributed) — retrieve and aggregate this data.

Currently, neither the stored data nor the query result metadata carries information
about which IP address(es) an interface held at the time traffic was captured. This
study evaluates how to add that capability efficiently, given that interface IPs
change infrequently relative to the write-out cadence.

---

## 2. Relevant System Facts

### 2.1 Write-Out Interval

Flow data is aggregated and flushed to disk every **300 seconds (5 minutes)**, driven
by `DBWriteInterval` in `pkg/goDB/DBWorkManager.go`. Each flush produces one block
in every active `.gpf` column file for that interface. A full year of data for one
interface therefore produces roughly **105,120 blocks** (288 blocks/day × 365 days).

### 2.2 On-Disk Layout

```
<dbroot>/
├── summary.json              # Global stats (begin/end/flowcount/traffic per iface)
└── <iface>/                  # e.g. eth0/
    └── YYYY/
        └── MM/
            └── <epoch>_<stats-suffix>/    # one directory per day
                ├── .blockmeta             # binary per-block metadata
                ├── sip.gpf
                ├── dip.gpf
                ├── dport.gpf
                ├── proto.gpf
                ├── l7proto.gpf
                ├── bytes_sent.gpf
                ├── bytes_rcvd.gpf
                ├── pkts_sent.gpf
                └── pkts_rcvd.gpf
```

### 2.3 .blockmeta Format

A binary file per day directory containing:
- A header with version, block count, aggregate traffic counters
- Per-block entries: timestamp delta, packet/byte counts, IPv4/IPv6 entry counts

The format is tightly specified and versioned. Adding fields would require a format
version bump and migration code.

### 2.4 summary.json

A global JSON file at `<dbroot>/summary.json` with per-interface aggregate statistics.
It is rewritten atomically at each writeout and is not designed for per-block history.

### 2.5 Change Frequency of Interface IPs

Interface IP addresses are intrinsically low-frequency data:

| Scenario | Typical IP change frequency |
|---|---|
| Static server NIC | Once per years, or never |
| DHCP lease renewal | Every few hours to days |
| Interface reconfiguration | Ad-hoc, operator-driven |
| Container/VM ephemeral | Minutes to hours (but also short-lived interfaces) |

Against 288 blocks per day and potentially years of data, storing an IP address per
block would be **massively redundant** — in the typical static-server case, the same
address would be repeated 105,120+ times.

---

## 3. Design Requirements

1. **Accuracy**: A query over a time range `[First, Last]` must be able to determine
   which IP(s) the interface held at every point in that range.
2. **Efficiency**: Storage overhead must scale with the number of IP changes, not
   with the number of blocks or the duration of the dataset.
3. **Backward compatibility**: Existing `.gpf` files, `.blockmeta`, and `summary.json`
   must remain unchanged and parseable by older tooling.
4. **Simplicity**: The structure must be trivially queryable by both `goquery` and
   `global-query` without complex new logic.
5. **Multi-address support**: An interface can hold multiple addresses simultaneously
   (e.g. one IPv4 + one or more IPv6 global/link-local addresses).
6. **CIDR notation**: Prefix length should be stored alongside the address to convey
   subnet context.

---

## 4. Evaluated Options

### Option A — One IP entry per .gpf block

Append the active IP address(es) to each block's payload or to `.blockmeta`.

**Pros**: Self-contained per-block; no external lookup needed.

**Cons**: Catastrophic storage redundancy (same IP repeated 100k+ times/year per
interface). Requires `.blockmeta` format change, version bump, and migration. Breaks
any tool that reads `.blockmeta` without an update.

**Verdict**: Rejected. Violates the efficiency requirement by design.

---

### Option B — Extend summary.json with IP history

Add a per-interface `ip_history` array to the existing `summary.json`.

```json
{
  "interfaces": {
    "eth0": {
      "begin": 1705276800,
      "end": 1705449600,
      "ip_history": [
        {"t": 1700000000, "addrs": ["192.168.1.1/24"]},
        {"t": 1705300000, "addrs": ["192.168.1.5/24"]}
      ]
    }
  }
}
```

**Pros**: No new files; reuses existing infrastructure.

**Cons**: `summary.json` is a global file, rewritten atomically on every writeout.
Embedding per-interface change logs here couples two very different update frequencies
and risks unnecessary write amplification. Over multi-year deployments with many
interfaces the file grows unboundedly with no natural truncation point.

**Verdict**: Rejected. Mixes concerns; poor operational profile at scale.

---

### Option C — Encode current IP in the day-directory name suffix

The day directory name already encodes aggregate stats as a suffix
(`<epoch>_<NumV4>-<NumV6>-<NumDrops>-...`). The current IP could be appended.

**Pros**: No new files; visible in `ls` output.

**Cons**: Directory names are parsed as structured metadata; adding variable-length
CIDR strings makes parsing fragile. Directory rename at writeout close already carries
risk; longer names increase that risk. Does not support multiple simultaneous
addresses cleanly. Impossible to capture mid-day changes.

**Verdict**: Rejected. The directory naming scheme is not the right abstraction layer
for this data.

---

### Option D — New per-interface change-log file: `ipmeta.json`

A new file at the interface root (`<dbroot>/<iface>/ipmeta.json`) that acts as an
append-only, time-ordered change log. Each entry records the Unix timestamp at which
the set of assigned addresses was observed to change, plus the new address list.

```json
[
  {"t": 1700000000, "addrs": ["192.168.1.1/24", "fe80::1/64"]},
  {"t": 1705300000, "addrs": ["192.168.1.5/24", "fe80::1/64"]},
  {"t": 1709500000, "addrs": ["192.168.1.5/24", "2001:db8::a/48", "fe80::1/64"]}
]
```

**Storage cost**: With O(10) IP changes per interface per year and CIDR strings of
~50 bytes each, an interface accumulates roughly **1–2 KB/year** in this file,
independent of data volume or block count.

**Query cost**: Reading the full file is O(n_changes); a binary search over the
timestamp array finds the active address set for any point in time in O(log
n_changes). For a time range `[First, Last]`, a linear scan from the largest entry
with `t ≤ First` to the first entry with `t > Last` yields all relevant snapshots.

**Write cost**: On each 5-minute writeout the capture manager already has the current
interface addresses (it needs them to correlate traffic). If the address set has not
changed since the previous writeout, nothing is written. If it has changed, a single
JSON object is appended — typically a single syscall.

**Pros**:
- Storage scales with IP-change frequency, not data volume
- Zero changes to `.gpf`, `.blockmeta`, or `summary.json`
- Fully backward-compatible: old tooling ignores the file
- Human-readable; trivially inspectable with any text editor or `jq`
- Supports multiple simultaneous addresses and CIDR notation naturally
- Captures sub-day changes (the timestamp resolution matches the writeout interval)
- Clean separation: flow data vs. interface metadata are distinct concerns

**Cons**:
- Introduces a new file that must be maintained over interface renames or DB copies
- JSON parsing is slightly slower than binary for very large files, but at O(10–100)
  entries/year this is negligible
- No built-in integrity checking (can be mitigated with a simple checksum field if
  needed)

**Verdict: Recommended.** Best fit for all stated requirements.

---

### Option E — Binary change-log (ipmeta.bin)

A binary-encoded variant of Option D: fixed-size records of
`[uint64 timestamp][uint8 num_addrs][per-addr: uint8 af, 4 or 16 bytes addr, uint8 prefix_len]`.

**Pros over D**: Slightly more compact; O(1) parsing per record without JSON overhead.

**Cons vs D**: Not human-readable; requires a dedicated parser; format evolution is
harder. Given the tiny file sizes involved (< 2 KB/year), the compactness gain is
negligible.

**Verdict**: Not recommended unless profiling reveals JSON parsing as a bottleneck,
which is implausible at the scale involved.

---

## 5. Recommended Design: `ipmeta.json`

### 5.1 File Location and Lifecycle

```
<dbroot>/
└── eth0/
    ├── ipmeta.json          ← new file
    ├── 2024/
    │   └── ...
    └── 2025/
        └── ...
```

The file is created on the first writeout for an interface and lives for the lifetime
of that interface's database directory.

### 5.2 Record Format

```json
[
  {
    "t":     1700000000,
    "addrs": ["192.0.2.1/24", "2001:db8::1/48", "fe80::1/64"]
  }
]
```

| Field   | Type            | Description                                     |
|---------|-----------------|--------------------------------------------------|
| `t`     | int64 (unix)    | Timestamp of the observation (writeout boundary) |
| `addrs` | []string (CIDR) | All addresses assigned to the interface          |

Records are sorted by `t` in ascending order. The array is the complete file content;
no outer envelope is required.

### 5.3 Concurrency and File Access

Understanding the existing locking model is essential before adding a new file.

#### What the codebase already does

| File | Write strategy | Read strategy | Locking |
|---|---|---|---|
| `.gpf` | Append-only; existing data never modified | Open + sequential read | None — append atomicity is sufficient |
| `.blockmeta` | Write to temp file, then `rename(2)` | Open + full read | None — atomic rename guarantees complete file |
| `summary.json` | Documented as using `O_EXCL\|O_CREAT` lock file | (same) | Documented but not currently implemented |

The dominant pattern for files that must be rewritten is **write-to-temp → `rename(2)`**
(`GPDir.writeMetadataAtomic()`). On Linux/POSIX `rename(2)` is atomic: any reader that
opens the path sees either the old complete file or the new complete file — never a
partial state. A reader that already has the old inode open continues reading the old
data unaffected; both sides are safe.

goquery holds **no locks** while reading. It opens files, reads them, and closes them.
This is safe for `.blockmeta` precisely because of the atomic-rename guarantee.

#### Why a naive append is not safe for `ipmeta.json`

A JSON array cannot be extended with a single atomic `write(2)` call once it grows
beyond a page. A reader catching a write mid-flight would see a truncated or
syntactically invalid array. Unlike `.gpf` files — where the columnar binary format
tolerates a partially written trailing block by checking block lengths — a JSON parser
will simply error.

#### Adopted strategy: atomic rename, consistent with `.blockmeta`

On each IP-change event the writer:

1. Serializes the complete updated array into a temp file in the same directory
   (e.g. `.ipmeta-<pid>.tmp`)
2. Calls `rename(2)` to replace `ipmeta.json` atomically
3. Deletes the temp file only on failure

Readers (goquery, global-query) open `ipmeta.json` and read it without acquiring any
lock. Because the file is always a complete, valid JSON array (or absent), no
synchronization primitive is needed on the read side — exactly the same guarantee
`.blockmeta` relies on.

**Cost**: the file is rewritten in full on each IP change. At O(10) changes/year and a
file size of ~2 KB after a decade, this is entirely negligible.

**No `ipmeta.lock` file is needed.** The `summary.lock` approach (documented but not
implemented) exists for a mutable file rewritten on every writeout under potential
concurrent access from multiple writers. `ipmeta.json` has a single writer (the
capture daemon) and is rewritten only on IP changes, so `rename(2)` atomicity alone is
sufficient.

### 5.4 Write Protocol

At each 5-minute writeout the capture manager:

1. Reads the current address list from the OS (already available during capture setup)
2. Compares it against the last-written entry cached in memory (no disk read after
   startup)
3. If unchanged: no write, no disk activity
4. If changed (or no file yet): serialize the full updated array to a temp file, then
   `rename(2)` it to `ipmeta.json`; update the in-memory cache

The initial record written at daemon startup always captures the address list at
startup time, establishing the baseline even if no subsequent changes occur.

### 5.5 Query Integration

#### goquery (local)

When building a query result, the query engine:

1. Opens `<dbroot>/<iface>/ipmeta.json` (or returns an empty list gracefully if absent)
2. Finds all entries with `t ≤ Last`; among those, the active entry at `First` is the
   last one with `t ≤ First`
3. Returns the list of distinct address snapshots observed during `[First, Last]` as
   part of the result metadata

This can be surfaced as:
- A new metadata field in JSON/CSV output (e.g. `interface_addrs`)
- A dedicated `--iface-info` flag that prints address history alongside query results

#### global-query (distributed)

Each sensor's query API response already includes per-interface metadata. The
`InterfaceMetadata` type can be extended with an `AddrHistory` field containing the
filtered snapshots. The aggregating server merges these per sensor, keyed by
`(sensor, interface)`.

### 5.6 Edge Cases

| Scenario | Handling |
|---|---|
| Interface has no IP (e.g., raw capture on an unnumbered link) | Store empty `addrs: []`; this is a valid state |
| Interface brought down and back up with same IP | No new record needed if address set is identical |
| Multiple writeouts before first IP change | Only the initial startup record is written |
| `ipmeta.json` absent (legacy database) | Query returns empty/unknown; no error |
| Database copied to another host | File is preserved with original timestamps — historically correct |
| Interface renamed | New interface directory means new (initially empty) `ipmeta.json`; old history is under the old name |

### 5.7 Retention and Compaction

Because the file grows at O(n_changes) and typical change rates are very low, no
compaction is needed in practice. If a policy is desired (e.g., drop entries older
than the oldest retained flow data), a maintenance pass can trim the array head
without affecting correctness of any remaining blocks.

---

## 6. Storage Estimate

| Parameter | Value |
|---|---|
| IP changes per interface per year (typical) | 5–20 |
| Average bytes per record (2 addrs, CIDR strings) | ~80 bytes |
| File size after 1 year | ~1.6 KB |
| File size after 10 years | ~16 KB |
| Overhead vs. flow data | Negligible (flow data: GB–TB scale) |

---

## 7. Summary

The recommended approach is a new per-interface **`ipmeta.json`** change-log file
stored at `<dbroot>/<iface>/ipmeta.json`. It records only the timestamps at which the
interface's address set changed, making storage cost proportional to change frequency
rather than data volume. The format is human-readable JSON, backward-compatible with
all existing tooling, and trivially queryable by both `goquery` and `global-query`
with a simple binary search or linear scan over a file that will remain small for the
lifetime of any realistic deployment.

No changes to `.gpf` files, `.blockmeta`, or `summary.json` are required.
