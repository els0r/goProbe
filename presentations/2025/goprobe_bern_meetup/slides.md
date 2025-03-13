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
    <div>Internet Traffic</div>
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

#### but in a simple style."

_— Oliver Goldsmith_

---

## Meet `eloc`

A markdown presentation authoring theme

---

for presenters who

1. __focus__ on writing
2. present in a __concise__ style

---

**`npm i slidev-theme-eloc`**

```yaml
# slides.md
---
theme: eloc
---

Just change the theme in your slide's frontmatter
```

---

<kbd>D</kbd> for __DARK MODE__

---

<kbd>O</kbd> for __OVERVIEW__

---

Click <kbd><carbon-text-annotation-toggle/></kbd> to __edit me__ :)

<div class="absolute bottom-14 left-85">
  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 254 262" class="w-[250px] h-[200px]">
    <g
      stroke-linecap="round" stroke="#999" stroke-width="1.5"
      transform="translate(244.6 10.8) rotate(0 -117.2 120.5)"
    >
      <path d="M0.0 -0.8 C-38.8 39.5, -195.7 201.6, -234.6 242.0" stroke-dasharray="8 9"></path>
      <path d="M-223.8 213.2 C-224.7 221.7, -228.6 226.4, -232.6 242.4"></path>
      <path d="M-209.1 227.4 C-213.1 232.9, -220.2 234.4, -232.6 242.4"></path>
    </g>
  </svg>
</div>

<style>
  .slidev-layout {
    .slidev-icon {
      @apply align-middle;
    }
    kbd:has(> .slidev-icon:only-child) {
      padding-inline: 0.2em 0.15em;
    }
  }
</style>

---

## Customization

```markdown
### Write inline style within markdown

<style>
  .slidev-layout {
    &::before {
      content: '';  background: center/cover url(...);
      /* you can use full ability with UnoCSS  */
      @apply absolute block -z-1 w-screen h-screen min-w-full min-h-full;
    }
    pre { opacity: 0.8 }
  }
</style>
```

<style>
  .slidev-layout {
    @apply overflow-visible;
    filter: invert();

    pre {
      @apply opacity-80;
    }

    &::before {
      @apply absolute block -z-1 w-screen h-screen min-w-full min-h-full;
      content: '';
      filter: invert();
      background: center/cover url(https://el-capitan.now.sh);
    }
  }
</style>

<!--
  bypass transform to scoped style in slidev
  https://github.com/slidevjs/slidev/blob/v51.1.1/packages/slidev/node/syntax/transform/in-page-css.ts#L15-L16
-->
<style no-scoped>
  #slide-content {
    @apply overflow-visible;
  }
</style>

---
background: ./screenshots/bg-initial.png
---

## Background

Just change in slide's frontmatter, like:

<div class="flex justify-center items-start gap-8">

```yaml
---
background: https://el-capitan.now.sh
---

# or pure color

---
background: #999
---
```

```yaml
---
background:
  # `image: <image url> or <gradient>`
  # or `color: <color>`
  image: https://el-capitan.now.sh
  dim: true  # dim color for image
  # invert content color on background
  invertContent: true
---
```

</div>

<style>
  .slidev-layout {
    pre {
      @apply opacity-80;
    }
    p {
      @apply mt-0;
    }
  }
</style>

---
layout: two-cols
class: flex flex-col justify-center items-center
---

<p>
  <a href="https://github.com/amio/eloc" target="_blank" rel="noopener">
    <img
      alt="amio"
      src="https://avatars.githubusercontent.com/u/215282"
      class="mb-2 w-52 h-52 rounded-full border-2 border-gray-300"
    />
  </a>
</p>

[![amio](https://badgen.net/badge/github/amio/blue?icon=github&label&scale=2)](https://github.com/amio)

[ [amio/eloc](https://github.com/amio/eloc) ]

[ [eloc.vercel](https://eloc.vercel.app) ]

::right::

<p>
  <a href="https://github.com/zthxxx" target="_blank" rel="noopener">
    <img
      alt="zthxxx"
      src="https://avatars.githubusercontent.com/u/15135943"
      class="mb-2 w-52 h-52 rounded-full border-2 border-gray-300"
    />
  </a>
</p>

[![zthxxx](https://badgen.net/badge/github/zthxxx/blue?icon=github&label&scale=2)](https://github.com/zthxxx)

[ [slidev-theme-eloc](https://www.npmjs.com/package/slidev-theme-eloc) ]

[ [eloc-slidev.vercel](https://eloc-slidev.vercel.app) ]

<style>
  .slidev-layout {
    p {
      margin-block: 1rem;
    }
  }
</style>
