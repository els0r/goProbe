---
# https://github.com/slidevjs/slidev/blob/v51.1.1/packages/types/src/config.ts#L10
theme:  apple-basic
layout: intro-image
image: ./pictures/bg-initial.png
title: Global Network Observability with goProbe
---

## Global Network Observability with `goProbe` and `goQuery`

_Bärner Go Meetup, 27.03.2025_

<br/>

Lennart Elsen

Fabian Kohn

<br/>

**Observability Team @ Open Systems AG**


---
layout: image-right
image: ./pictures/els0r-gh.png
---

# Lennart Elsen

Systems/Software Engineer at Open Systems
> Observability, Fleet Management, Traffic Analysis, `golang`

Born and raised in Hamburg, Germany
> Zurich, ZH, CH

Surfing, Coffee and Open Source Software
> South Shore Beach, RI, US, Double Espresso (no cream, no sugar), [els0r/goProbe](https://github.com/els0r/goProbe)

---
layout: image-right
image: ./pictures/fako1024-gh.png
---

# Fabian Kohn

Systems/Software Engineer at Open Systems
> Performance Optimization, High-Energy Physics, Traffic Analysis, `golang`

Born and raised in Göttingen, Germany
> Hamburg, HH, DE

Running, Coffee and Open Source Software
> Everywhere, Flat White, [fako1024/slimcap](https://github.com/fako1024/slimcap)


---
layout: default
---

<div class="flex h-screen justify-center">
  <div class="w-1/3 flex flex-col items-center">
    <div>Internet Traffic</div>dd
    <img src="./pictures/packets_single.png" alt="Internet Traffic" class="mt-4 h-48">
  </div>
  <div class="w-1/3 flex flex-col items-center">
    <div></div>
  </div>
  <div class="w-1/3 flex flex-col items-center">
    <div></div>
  </div>
</div>

---
layout: default
---

<div class="flex h-screen justify-center">
  <div class="w-1/3 flex flex-col items-center">
    <div>Internet Traffic</div>
    <img src="./pictures/packets_single.png" alt="Internet Traffic" class="mt-4 h-48">
  </div>
  <div class="w-1/3 flex flex-col items-center">
    <div></div>
  </div>
  <div class="w-1/3 flex flex-col items-center">
    <div>Customer</div>
    <img src="./pictures/hosts.png" alt="Customer" class="mt-4 h-48">
  </div>
</div>

---
layout: default
---

<div class="flex h-screen justify-center">
  <div class="w-1/3 flex flex-col items-center">
    <div>Internet Traffic</div>
    <img src="./pictures/packets_single.png" alt="Internet Traffic" class="mt-4 h-48">
  </div>
  <div class="w-1/3 flex flex-col items-center">
    <div>Open Systems</div>
    <img src="./pictures/os.png" alt="Open Systems" class="mt-4 h-48">
  </div>
  <div class="w-1/3 flex flex-col items-center">
    <div>Customer</div>
    <img src="./pictures/hosts.png" alt="Customer" class="mt-4 h-48">
  </div>
</div>

---
layout: default
---

<div class="flex h-screen justify-center">
  <div class="w-1/3 flex flex-col items-center">
    <div class="text-center">What's the traffic composition?</div>
    <img src="./pictures/packets_single.png" alt="Internet Traffic" class="mt-4 h-48">
  </div>
  <div class="w-1/3 flex flex-col items-center opacity-20">
    <div class="opacity-0">Open Systems</div>
    <img src="./pictures/os.png" alt="Open Systems" class="mt-4 h-48">
  </div>
  <div class="w-1/3 flex flex-col items-center opacity-20">
    <div class="opacity-0">Customer</div>
    <img src="./pictures/hosts.png" alt="Customer" class="mt-4 h-48">
  </div>
</div>

---
layout: image
---

# An IP packet

![](./pictures/packet_detail.png)

---
layout: default
---

# For `t == time.Now()`

Live capture

```shell
tcpdump -ni eth0
```

---
layout: default
---

# For `t == time.Now()`

Live capture

```shell
tcpdump -ni eth0
```

Output

```shell
tcpdump: verbose output suppressed, use -v[v]... for full protocol decode
listening on eth0, link-type EN10MB (Ethernet), snapshot length 262144 bytes
11:33:16.002178 IP 211.154.236.12.35178 > 10.236.2.18.22: Flags [.], ack 188, win 83, options [nop,nop,TS val 515841640 ecr 3570605299], length 0
11:33:16.021053 IP 211.154.236.12.35178 > 10.236.2.18.22: Flags [P.], seq 1:37, ack 188, win 83, options [nop,nop,TS val 515841659 ecr 3570605299], length 36
11:33:16.021268 IP 10.236.2.18.22 > 211.154.236.12.35178: Flags [P.], seq 188:224, ack 37, win 83, options [nop,nop,TS val 3570605320 ecr 515841659], length 36
```

---
layout: default
---

# For `t == now`

Live capture

```shell
tcpdump -ni eth0
```

Output

```shell
tcpdump: verbose output suppressed, use -v[v]... for full protocol decode
listening on eth0, link-type EN10MB (Ethernet), snapshot length 262144 bytes
11:33:16.002178 IP 211.154.236.12.35178 > 10.236.2.18.22: Flags [.], ack 188, win 83, options [nop,nop,TS val 515841640 ecr 3570605299], length 0
11:33:16.021053 IP 211.154.236.12.35178 > 10.236.2.18.22: Flags [P.], seq 1:37, ack 188, win 83, options [nop,nop,TS val 515841659 ecr 3570605299], length 36
11:33:16.021268 IP 10.236.2.18.22 > 211.154.236.12.35178: Flags [P.], seq 188:224, ack 37, win 83, options [nop,nop,TS val 3570605320 ecr 515841659], length 36
```

What a network engineer looks at

```shell
11:33:16.002178 SrcIP 211.154.236.12         > DstIP 10.236.2.18    Port 22
11:33:16.021053 SrcIP 211.154.236.12         > DstIP 10.236.2.18    Port 22
11:33:16.021268 SrcIP 10.236.2.18    Port 22 > DstIP 211.154.236.12
```

_Bi-directional traffic (SSH session) from 211.154.236.12 to 10.236.2.18_

---
layout: fact
---

_Bi-directional traffic_

_(SSH session, TCP port 22)_

_from 211.154.236.12_

_to 10.236.2.18_

---
layout: default
---

# For `t == now - 24h`?


```shell
goquery -i eth0 -f -24h -c "dip=10.236.2.18 and sip=211.154.236.12 and dport=22 and proto=tcp" sip,dip,dport,proto
```

Yields

```
                                             packets  packets             bytes      bytes
             sip          dip  dport  proto       in      out       %        in        out       %
  211.154.236.12  10.236.2.18     22    TCP    481      475    100.00  59.27 kB   65.91 kB  100.00

                                               481      475            59.27 kB   65.91 kB

         Totals:                                        956                      125.18 kB

Timespan    : [2025-03-10 11:47:03, 2025-03-11 11:50:00] (1d3m0s)
Interface   : eth0
Sorted by   : accumulated data volume (sent and received)
Conditions  : (dip = 10.236.2.18 & (sip = 211.154.236.12 & (dport = 22 & proto = tcp)))
Query stats : displayed top 1 hits out of 1 in 9ms
```

---

# Hello `slimcap`

```go
func StartInterface(ctx context.Context) {
  return
}
```

---

## Next-Gen Packet Capture
### Goals / DoD

Resource limitations running goProbe on several hosts

Existing capture solution:
* Does a lot *[more than we need]* under the hood
* Complex / intricate to use (stateful pcap capture handle)
* Customizations / fork required

C(GO) / system library dependency (`libpcap`)

Abysmal testing capabilities

---
layout: two-cols
---

## Next-Gen Packet Capture
### Goals / DoD

Minimize Overhead:
* IP Layer extraction (if exists)
* Limit to start of transport layer

Focus on Linux (but keep extensible)

Native Go without external *[read: C(GO)]* dependencies

Ease of use (semi-stateless)

Zero-copy / zero-allocation support

Out-of-the-box tests / benchmarks

::right::

<div class="flex justify-center items-center">
 <div class="translate-y-[80%]">
  <img src="./pictures/slimcap/layers.png">
 </div>
</div>

---

## Capture Setup `AF_PACKET & MMAP()`

<div class="flex justify-center items-center">
 <div class="w-[75%] translate-y-[3%]">
  <img src="./pictures/slimcap/ringbuf_1.png">
 </div>
</div>

---

## Capture Setup `AF_PACKET & MMAP()`

<div class="flex justify-center items-center">
 <div class="w-[75%] translate-y-[3%]">
  <img src="./pictures/slimcap/ringbuf_2.png">
 </div>
</div>

---

## Capture Setup `AF_PACKET & MMAP()`

<div class="flex justify-center items-center">
 <div class="w-[75%] translate-y-[3%]">
  <img src="./pictures/slimcap/ringbuf_3.png">
 </div>
</div>

---

# Interfaces

````md magic-move
```go
// Source denotes a generic packet capture source
type Source interface {
    NextPayload(pBuf []byte) ([]byte, byte, uint32, error)
    NextIPPacket(pBuf IPLayer) (IPLayer, PacketType, uint32, error)
    Stats() (Stats, error)
    Close() error
    // ...
}
```
```go
// SourceZeroCopy denotes a capture source that supports zero-copy operations
type SourceZeroCopy interface {
    NextPayloadZeroCopy() ([]byte, PacketType, uint32, error)
    NextIPPacketZeroCopy() (IPLayer, PacketType, uint32, error)

    // Wrap generic Source
    Source
}
```
````

---

# Minimal Example

````md magic-move
```go {1,2,8}
src, err := afring.NewSource(
    “enp1s0”,
    afring.CaptureLength(link.CaptureLengthMinimalIPv6Transport),
    afring.BufferSize(
        1024*1024,       // Block Size
        4,               // Number of Blocks
    ),
)

if err != nil {
    // Error handling
}
```
```go {1,3,8}
src, err := afring.NewSource(
    “enp1s0”,
    afring.CaptureLength(link.CaptureLengthMinimalIPv6Transport),
    afring.BufferSize(
        1024*1024,       // Block Size
        4,               // Number of Blocks
    ),
)

if err != nil {
    // Error handling
}
```
```go {1,4-8}
src, err := afring.NewSource(
    “enp1s0”,
    afring.CaptureLength(link.CaptureLengthMinimalIPv6Transport),
    afring.BufferSize(
        1024*1024,       // Block Size
        4,               // Number of Blocks
    ),
)

if err != nil {
    // Error handling
}
```
```go
src, err := afring.NewSource(
    “enp1s0”,
    afring.CaptureLength(link.CaptureLengthMinimalIPv6Transport),
    afring.BufferSize(
        1024*1024,       // Block Size
        4,               // Number of Blocks
    ),
)

if err != nil {
    // Error handling
}
```
````

---

# Minimal Example (cont'd)

````md magic-move
```go
for {
    ipLayer, pktType, pktLen, err := src.NextIPPacketZeroCopy()
    if err != nil {
        if errors.Is(err, capture.ErrCaptureStopped) {
            // Graceful stop
            break
        }
        // Error handling
    }
}
```
```go
for {
    ipLayer, pktType, pktLen, err := src.NextIPPacketZeroCopy()
    if err != nil {
        if errors.Is(err, capture.ErrCaptureStopped) {
            // Graceful stop
            break
        }
        // Error handling
    }

    // Do stuff ...
    _ = ipLayer        // Raw IP layer data (up to snaplen)
    _ = pktType        // Packet Type (direction flag)
    _ = pktLen         // Total packet length
}
```
````

---

## Testing
### Mock Capture Sources

Stand-in wrappers (down to socket interaction) around actual sources:
* `AF_PACKET` socket vs. simple FD / EFD semaphore
* `MMAP`’ed area vs. user space slice
* Memory barrier vs. atomic status flag / field

<div class="flex justify-center items-center">
 <div class="w-[90%] translate-y-[20%]">
  <img src="./pictures/slimcap/mock.png">
 </div>
</div>

---

## Testing
### Mock Capture Sources

Stand-in wrappers (down to socket interaction) around actual sources:
* `AF_PACKET` socket vs. simple FD / EFD semaphore
* `MMAP`’ed area vs. user space slice
* Memory barrier vs. atomic status flag / field

Features:
* Reading / replay of pcap dumps (no timing)
* Synthetic packet / payload generation
* No privileges / *actual* interfaces
* Piping from other mock sources
* High-throughput mode (benchmarks)

---

## Testing
### Benchmarks

**Testbed:** Quad-core Odroid H3, 32 GiB RAM

**Scenario:** Synthetic mock benchmark (zero-copy packet retrieval) on `slimcap`

<div class="w-[45%] translate-x-[10%] translate-y-[20%] grid gap-1">
  <div class="grid grid-cols-3">
    <span>Time / op:</span>
    <span class="text-right">16.2 ns</span>
    <span class="text-right color-coolgray">± 1%</span>
  </div>
  <div class="grid grid-cols-3">
    <span>Throughput:</span>
    <span class="text-right">61.7 Mpps</span>
    <span class="text-right color-coolgray">± 1%</span>
  </div>
  <div class="grid grid-cols-3">
    <span>Allocations / op:</span>
    <span class="text-right">0</span>
    <span class="text-right"></span>
  </div>
</div>

---

## Testing
### Benchmarks

**Testbed:** TBD

**Scenario:** 1h Real-life capture `goProbe` v3 (`gopacket`) / v4 (`slimcap`)

<div class="w-[70%] translate-x-[10%] translate-y-[20%] grid gap-1">
  <div class="grid grid-cols-[1.5fr_1fr_1fr_1fr_1fr]">
    <span>CPU Time:</span>
    <span class="text-right">XXX s</span>
    <span class="text-right">vs.</span>
    <span class="text-right">YYY s</span>
    <span class="text-right color-blue">~ xZZ</span>
  </div>
  <div class="grid grid-cols-[1.5fr_1fr_1fr_1fr_1fr]">
    <span>Peak Mem Usage:</span>
    <span class="text-right">XXX MiB</span>
    <span class="text-right">vs.</span>
    <span class="text-right">YYY MiB</span>
    <span class="text-right color-blue">~ xZZ</span>
  </div>
  <div class="grid grid-cols-[1.5fr_1fr_1fr_1fr_1fr]">
    <span>Dropped Packets:</span>
    <span class="text-right">XXX</span>
    <span class="text-right">vs.</span>
    <span class="text-right">YYY</span>
    <span class="text-right color-blue">~ xZZ</span>
  </div>
</div>
