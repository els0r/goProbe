# Research: Adapting goProbe / slimcap to VPP (Vector Packet Processing)

**Status:** Research only. No implementation proposed here.
**Scope:** Facts and trade-offs for plugging a VPP-based packet source into goProbe via the slimcap `Source` abstraction.

---

## 1. Background

### 1.1 Current capture path in goProbe

goProbe captures packets through the [slimcap](https://github.com/fako1024/slimcap) library. The production source is `afring`, an AF_PACKET TPACKET_V3 MMAP ring buffer.

Relevant files:

- `pkg/capture/capture.go:34-42` — default source factory wires `afring.NewSource(...)` with capture length, block size, number of blocks, promiscuous mode, VLAN handling and extra BPF filters.
- `pkg/capture/capture.go:73-76` — `Capture` holds an abstract `Source` plus a `sourceInitFn` that can be overridden (used today only for mock tests).
- `pkg/capture/capture.go:262` — hot loop calls `c.captureHandle.NextIPPacketZeroCopy()` in a tight `for` loop; the only special returns handled are `ErrCaptureUnblocked` and `ErrCaptureStopped`.
- `pkg/capture/capture_mock.go:9` vs `pkg/capture/capture_nomock.go:9` — build-tag switch that makes `Source` either the slimcap `capture.SourceZeroCopy` interface (default, allows mocks) or the concrete `*afring.Source` (when tag `slimcap_nomock` is set for max performance).

### 1.2 The slimcap `Source` abstraction

slimcap exposes two interfaces:

```
Source
    NewPacket() Packet
    NextPacket(pBuf Packet) (Packet, error)
    NextPayload(pBuf []byte) ([]byte, PacketType, uint32, error)
    NextIPPacket(pBuf IPLayer) (IPLayer, PacketType, uint32, error)
    NextPacketFn(func(...) error) error
    Stats() (Stats, error)
    Link() *link.Link
    Unblock() error
    Close() error

SourceZeroCopy (superset)
    NextPayloadZeroCopy() ([]byte, PacketType, uint32, error)
    NextIPPacketZeroCopy() (IPLayer, PacketType, uint32, error)
```

Existing implementations inside slimcap:

| Implementation                        | Purpose                                 |
|---------------------------------------|-----------------------------------------|
| `capture/afpacket/afring.Source`      | TPACKET_V3 MMAP ring buffer (prod)     |
| `capture/afpacket.Source`             | classic AF_PACKET (fallback)           |
| `capture/pcap.Source`                 | pcap file replay (offline/test)        |
| `afring.MockSource` / `MockSourceNoDrain` | in-process mock used in tests    |

### 1.3 What VPP is, briefly

VPP (FD.io Vector Packet Processing) is a userspace forwarding plane. NICs are bound to DPDK/RDMA drivers and leave the kernel. Because the kernel no longer sees the wire, any tool that relies on AF_PACKET (including goProbe today) sees **zero** traffic for those interfaces. To observe traffic on a VPP dataplane, packets must be extracted through one of VPP’s external interfaces:

1. **memif** — the canonical VPP packet-based shared-memory interface.
2. **tap / tun** — VPP creates a kernel tap, traffic is forked/punted through it. AF_PACKET works against that tap.
3. **pcap trace / dispatch trace** — file-based tracing, not suitable for continuous monitoring.
4. **AF_XDP / AF_PACKET fanout mirror** — only works if the NIC is still visible to the kernel, which generally contradicts running VPP on that NIC.
5. **IPFIX `flowprobe` plugin** — VPP produces IPFIX records natively; this is a completely different pipeline (collector model) and sidesteps goProbe's aggregation.

For a "capture source" that is conceptually equivalent to afring, the two realistic options are **memif** and **tap**.

---

## 2. Option A — Consume VPP via a Linux tap

VPP can be configured with `create tap` (or `create host-interface` / punt) to mirror or hand off packets to a Linux kernel interface. goProbe/slimcap then attach with the existing `afring` source on that tap.

Properties:

- **No code changes in slimcap or goProbe.** Configuration change only: point goProbe at the tap name.
- **Double copy**: packet traverses VPP → kernel tap → MMAP ring. Performance is the kernel’s tap throughput — typically far below what VPP itself can push.
- Safety is unchanged from today’s AF_PACKET story: kernel owns the ring, userspace (goProbe) only reads.
- Bypass risk: if VPP owns the NIC exclusively, the operator must deliberately punt or mirror traffic onto the tap; forgetting this makes goProbe report empty flows.

This option is included for completeness. It is an integration/deployment pattern, not a slimcap change.

---

## 3. Option B — Native memif source in slimcap

This is what the guiding questions point at. memif is a "packet-based shared memory interface for userspace processes" (the VPP equivalent of vhost-user, purpose-built for packet I/O rather than virtio devices).

### 3.1 Protocol summary

- Two roles per interface: **master** and **slave**. Exactly one of each per memif link.
- Control plane: UNIX domain socket. Negotiates regions, rings, ring size, packet buffer size, optional 24-byte shared secret.
- Data plane: one or more **shared memory regions** (mmap'd files) containing:
  - **S2M rings** (slave→master),
  - **M2S rings** (master→slave),
  - packet **buffers** addressed by `(region_index, offset, length)` descriptors.
- Notification: per-queue **eventfd**; can also be polled.
- Producer/consumer: **slave creates and owns the shared memory file**; master mmap’s it. This asymmetry matters (see §3.4).
- Zero-copy is possible but has a footnote: the DPDK memif PMD only supports zero-copy **on the slave side** and requires DPDK EAL `--single-file-segments` so that packet buffers can be addressed by offset into a single hugepage file. VPP’s own memif implementation has similar constraints. The Go reference library (`github.com/fdio/vpp/extras/gomemif/memif`) exposes `ReadPacket([]byte)` / `WritePacket([]byte)`, which **copy** between the ring and a caller-provided buffer; its zero-copy path is not part of the public Go API.

### 3.2 Shape of a hypothetical `capture/memif.Source` in slimcap

To satisfy slimcap's `SourceZeroCopy`, an implementation would need to:

1. Own a `*memif.Socket` and a `*memif.Interface` (one per goProbe interface name).
2. Role decision: **act as master** listening on a socket path; VPP is configured with the corresponding `memif` interface as slave (VPP can be either, but slave-side zero-copy is only useful when VPP is the slave — which is the conventional direction). Alternative: goProbe as slave, VPP as master — symmetrical but inverts memory ownership.
3. On `ConnectedFunc` callback, acquire RX queues via `iface.GetRxQueue(qid)` and pull the queue’s eventfd for blocking.
4. Implement `NextIPPacketZeroCopy()` / `NextPayloadZeroCopy()`:
   - Block on queue eventfd (analogous to afring’s PPOLL).
   - Pop the next RX ring descriptor.
   - Return a slice that points **into the shared memory region** for the descriptor’s buffer offset and length.
   - The slice MUST be considered valid only until the next call — same contract slimcap already documents for afring zero-copy.
5. Implement `Unblock()` by writing to an internal wake eventfd or closing the queue eventfd, matching the semantics goProbe relies on in `capture.go:262-268`.
6. `Link()` returns a synthetic `*link.Link`. There is no kernel `if_index`; slimcap’s `link` package would need to accept a logical/virtual link or the implementation would have to fabricate one. This is the most awkward impedance mismatch.
7. `Stats()` maps memif counters (ring full/empty, drops) to `capture.Stats`.

Packet framing: memif transports **L2 frames by default** (Ethernet), which matches what afring delivers. goProbe’s `ParsePacketV4` / `ParsePacketV6` paths expect an IP layer — slimcap’s afring source already strips link headers inside `NextIPPacketZeroCopy`; the memif source would do the same, minus the VLAN-stripping edge cases that afring handles for the TPACKET_V3 ring format.

BPF filters: **not available.** memif is not a socket; the `afring.ExtraBPFInstructions` / `afring.CaptureLength` options have no analog. Any filtering must be done in Go after-the-fact or be expressed as a VPP graph node before the memif TX.

Promiscuous / VLAN / capture length: none apply. The interface receives whatever VPP sends; VPP’s graph decides.

### 3.3 Answers to the guiding questions

**Q1: Is this an additional source implementation from slimcap?**

Yes. The cleanest fit is a new package `github.com/fako1024/slimcap/capture/memif` that implements `capture.SourceZeroCopy`. On goProbe’s side the only change is the `defaultSourceInitFn` at `pkg/capture/capture.go:34` — either by switching on a config flag (`iface.kind = afpacket | memif`) or by allowing per-interface source factories. The `Source` type alias in `capture_mock.go` already keeps the rest of goProbe source-agnostic; a memif source drops in.

Caveats that *may* leak upward:

- `link.Link` currently exposes kernel-level attributes (if-index, MTU from netlink). A memif source needs a virtual link descriptor; slimcap’s link package would have to grow a "virtual" constructor, or goProbe would have to tolerate zero-valued link metadata.
- goProbe’s config struct `CaptureConfig.RingBuffer.{BlockSize,NumBlocks}` and `Promisc` / `IgnoreVLANs` / `ExtraBPFFilters` are afring-specific. They would need to become optional / ignored for memif interfaces, with memif-specific options (socket path, role, secret, queue count, ring size log2) living in a sibling sub-struct.

**Q2: Does this fundamentally change the capture loop in goProbe?**

No, not the loop itself. The hot loop in `pkg/capture/capture.go:234-312` is:

```
for {
    if c.capLock.HasLockRequest() { ... bufferPackets ... continue }
    ipLayer, pktType, pktSize, err := c.captureHandle.NextIPPacketZeroCopy()
    // err handling for ErrCaptureUnblocked / ErrCaptureStopped
    // parse IPv4 / IPv6, add to flow log
}
```

Every primitive used — `NextIPPacketZeroCopy`, `ErrCaptureUnblocked`, `ErrCaptureStopped`, `Unblock()`, `Close()` — is already part of the `SourceZeroCopy` contract. As long as the memif source honours that contract (including `Unblock()` returning promptly so the three-point rotation lock in `capLock` can progress), the loop is untouched.

Peripheral changes are small but real:

- Bootstrap: construction today is synchronous (`afring.NewSource` returns a ready source). memif is asynchronous — it is only usable after the `ConnectedFunc` callback fires. `newCapture` (`pkg/capture/capture.go:96`) would need to block on a readiness signal or mark the capture as "waiting" until VPP connects. `capture_manager.go` already tolerates errors from `sourceInitFn`; the simplest bridge is to block inside the source constructor until connected (with a timeout).
- Disconnects: unlike a kernel NIC which rarely disappears, memif master↔slave connections can bounce when VPP is restarted. The memif source must either auto-reconnect internally (hiding the event) or surface a specific error that goProbe treats like an unplug rather than a fatal capture error.
- Metrics: `capture.Stats` has `PacketsReceived` / `PacketsDropped` / `PacketsFreed`. memif naturally reports `ring_full_drops` and `no_buf_drops` — semantically close but not identical.

**Q3: How safe is it from the perspective of shared memory?**

The honest answer is: **considerably riskier than AF_PACKET, and the risk model must be understood before deploying.**

1. **Trust boundary moves from kernel↔user to peer↔peer.**
   With afring, the kernel writes packets into a ring it allocated itself and goProbe (userspace) only reads. A malicious userspace process can at worst corrupt its own view. With memif, two userspace processes (VPP and goProbe) share a writable memory file. Whoever is the **slave** created the file; whoever is the **master** mmap’d it. Both processes can, in principle, write anywhere in the region.

2. **Implication for buffer descriptor validation.**
   A memif consumer must treat every descriptor `(region_index, offset, length)` read from an RX ring as **untrusted input**. If the peer (accidentally or deliberately) sets an out-of-bounds offset or an oversized length, the consumer can read past the mmap’d region → SIGBUS, or read neighbouring packets → information leak across tenants. slimcap’s afring code does not need these checks because the kernel enforces them; a memif source must perform them on every single packet. This is the single biggest source of subtle bugs in memif consumers.

3. **Zero-copy and Go's memory model.**
   If a zero-copy memif source returns a `[]byte` that aliases shared memory, that slice is mutable by the peer until the descriptor is returned. goProbe currently treats the zero-copy IP layer as read-only and releases it implicitly on the next `NextIPPacketZeroCopy()` call — so the existing pattern is compatible, but anything that escapes the loop (e.g. if a future refactor captured the slice into a long-lived structure) would become a silent memory-safety hazard. A TOCTOU-style bug — parse the header, then the peer mutates the payload, then re-read — is possible in principle.

4. **Process privileges and socket placement.**
   The UNIX domain socket that bootstraps a memif connection is the real authentication boundary. Whoever can `connect()` or `listen()` on that socket path becomes a memif peer and gets a writable mapping to the region. Filesystem permissions on the socket directory are therefore critical. memif additionally supports an optional 24-byte shared `secret` — useful but not a substitute for filesystem permissions.

5. **Hugepages / mlock.**
   Production memif typically uses hugepage-backed shared files. These are not swappable; a misconfigured ring size multiplied by many interfaces can pin large amounts of RAM. Less of a safety issue, more of an operational footgun.

6. **No BPF safety net.**
   With afring, `ExtraBPFFilters` can discard unwanted traffic in-kernel before it ever reaches goProbe's address space. With memif, whatever VPP sends ends up in goProbe's shared region. Filter pushdown has to happen in the VPP graph (ACL/classifier node before the memif TX), otherwise every packet lands in user memory regardless.

7. **Rotation / `Unblock()` safety.**
   goProbe's three-point lock in `capLock` (`pkg/capture/capture.go:126-131`) assumes that once `Unblock()` returns `ErrCaptureUnblocked`, no further packet will be processed until the lock is released. For afring this is tied to the PPOLL wake-up. For memif, `Unblock()` must synchronize against the queue consumer goroutine and ensure descriptors the kernel-equivalent "return-to-peer" step is actually performed — otherwise descriptors leak and the ring eventually stalls. This is a correctness rather than safety concern but worth calling out because the existing lock protocol is sensitive to it.

### 3.4 Summary trade-off table

| Concern                       | afring (today)                         | memif source (hypothetical)                               |
|-------------------------------|----------------------------------------|-----------------------------------------------------------|
| New slimcap package needed    | —                                      | Yes, `capture/memif`                                      |
| goProbe hot-loop changes      | —                                      | None (same `SourceZeroCopy` contract)                     |
| goProbe peripheral changes    | —                                      | Config schema, bootstrap readiness, reconnect handling    |
| Zero-copy RX                  | Yes, kernel-enforced                   | Possible, but requires manual descriptor validation       |
| Filter pushdown               | BPF in kernel                          | Must be done in VPP graph                                 |
| Trust model                   | kernel ↔ user (asymmetric, enforced)   | user ↔ user (symmetric, cooperative)                      |
| Failure mode of bad peer      | N/A                                    | SIGBUS / info leak / ring stall if descriptors unchecked  |
| Bring-up                      | Synchronous at `NewSource()`           | Asynchronous; waits for VPP to connect                    |
| Interface metadata (`Link`)   | from netlink                           | must be synthesized                                       |
| VLAN / promisc / snap-len     | honoured by afring                     | N/A (VPP’s problem)                                       |
| Performance ceiling           | kernel AF_PACKET                       | higher, bounded by ring size × copy cost                  |

---

## 4. Recommendation for further investigation (non-binding)

If a VPP source is actually needed:

1. **Start with tap (Option A).** Zero code, valid baseline to measure what throughput is actually required before taking on the memory-safety burden of memif.
2. **Prototype `slimcap/capture/memif` with the copy-based API first** (`gomemif.Queue.ReadPacket([]byte)`). This keeps the trust model simple: the descriptor is consumed inside gomemif, only a copied buffer escapes. Performance is still a multiple of tap because there is only one copy and no kernel crossing.
3. **Only then consider a zero-copy memif path.** Gate it behind a build tag similar to `slimcap_nomock`, audit every offset/length use site, and document that the returned slice must not escape the loop.
4. Extend `CaptureConfig` with a discriminated union for source kind; leave afring as default.
5. Audit `pkg/capture/capture_manager.go` for assumptions about interfaces being kernel-visible (e.g. anything that calls netlink by interface name).

---

## Sources

- [slimcap — github.com/fako1024/slimcap](https://github.com/fako1024/slimcap)
- [gomemif — github.com/fdio/vpp/extras/gomemif/memif](https://pkg.go.dev/github.com/fdio/vpp/extras/gomemif/memif)
- [Memif library (libmemif) — VPP docs](https://s3-docs.fd.io/vpp/22.06/interfacing/libmemif/index.html)
- [DPDK Memif PMD — zero-copy slave requirements](https://doc.dpdk.org/guides/nics/memif.html)
- [govpp/extras/libmemif README](https://github.com/FDio/govpp/blob/master/extras/libmemif/README.md)
- [Punting Packets — VPP 20.05](https://fd.io/docs/vpp/v2005/gettingstarted/developers/punt.html)
- [VPP flowprobe (IPFIX) plugin](https://docs.fd.io/vpp/17.10/flowprobe_plugin_doc.html)
- [goProbe paper (INESC-ID PDF)](https://www.dpss.inesc-id.pt/~ler/docencia/rcs1617/papers/goprobe.pdf)
