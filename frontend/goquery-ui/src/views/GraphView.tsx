import React, { useMemo, useRef, useEffect, useState } from 'react'
import { FlowRecord } from '../api/domain'
import { formatBytesIEC } from '../utils/format'

export interface GraphViewProps {
  rows: FlowRecord[]
  loading: boolean
  // cap total number of nodes (unique IPs + interfaces + hosts)
  maxNodes: number
  onIpClick?: (ip: string) => void
  onIfaceClick?: (host: string, iface: string) => void
  onHostClick?: (hostId: string) => void
}

type NodeType = 'ip' | 'iface' | 'host'

interface Node {
  id: string
  label: string
  type: NodeType
  // grouping keys
  host?: string
  iface?: string
  hostId?: string
  // position (computed)
  x: number
  y: number
  // for host bubble sizing
  radius?: number
}

interface Edge {
  id: string
  from: string
  to: string
  color: string
  width: number
  title: string
  sip: string
  dip: string
  bytesTotal: number
  packetsTotal: number
}

// map a 0..1 value to stroke width 1..8px in 12.5% buckets
function edgeWidth01(v01: number): number {
  if (!isFinite(v01) || v01 <= 0) return 1
  const bin = Math.min(7, Math.floor(v01 * 8)) // 0..7
  return 1 + bin
}

const BLUE = '#60a5fa' // tailwind blue-400 (also used for edge/label accents)
const RED = '#f87171' // tailwind red-400 (matches table row accent)
const BLUE_STROKE = '#93c5fd' // blue-300 (legacy default)
// Stronger visual separation for IP node fills/strokes
const PUBLIC_FILL = '#3b82f6' // blue-500
const PUBLIC_STROKE = '#60a5fa' // blue-400
const PRIVATE_FILL = '#dbeafe' // blue-100
const PRIVATE_STROKE = '#bfdbfe' // blue-200
const HOST_FILL = 'rgba(59,130,246,0.12)' // translucent blue
const HOST_STROKE = 'rgba(255,255,255,0.08)'

// node radii (kept top-level so layout + render agree)
const IP_R = 16
const IFACE_R = 12

// simple responsive container size hook
function useRect(ref: React.RefObject<HTMLElement>) {
  const [rect, setRect] = useState<{ width: number; height: number }>({ width: 800, height: 500 })
  useEffect(() => {
    if (!ref.current) return
    const obs = new ResizeObserver(entries => {
      const r = entries[0].contentRect
      setRect({ width: Math.max(600, r.width), height: Math.max(400, r.height) })
    })
    obs.observe(ref.current)
    return () => obs.disconnect()
  }, [ref])
  return rect
}

export function GraphView({ rows, loading, maxNodes, onIpClick, onIfaceClick, onHostClick }: GraphViewProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const { width, height } = useRect(containerRef)
  const [hoverIP, setHoverIP] = useState<string | null>(null)
  const [panelBg, setPanelBg] = useState<string>('#0f172a')

  // detect nearest non-transparent background color to use as text outline fill
  useEffect(() => {
    let el: HTMLElement | null = containerRef.current
    let found: string | null = null
    try {
      while (el) {
        const bg = getComputedStyle(el).backgroundColor
        if (bg && bg !== 'rgba(0, 0, 0, 0)' && bg !== 'transparent') { found = bg; break }
        el = el.parentElement as HTMLElement | null
      }
    } catch {
      // ignore
    }
    if (found) setPanelBg(found)
  }, [])
  const [hoverIface, setHoverIface] = useState<string | null>(null)

  // classify private IPs (IPv4 RFC1918 A/B/C + loopback/link-local; IPv6 ULA/link-local)
  const isPrivateIP = (ip: string): boolean => {
    if (!ip) return false
    if (ip.includes(':')) {
      const lower = ip.toLowerCase()
      return lower.startsWith('fc') || lower.startsWith('fd') || lower.startsWith('fe80') || lower === '::1'
    }
    const m = ip.match(/^(\d{1,3})\.(\d{1,3})\./)
    if (!m) return false
    const a = parseInt(m[1], 10)
    const b = parseInt(m[2], 10)
    if (a === 10) return true // 10.0.0.0/8 (Class A private)
    if (a === 172 && b >= 16 && b <= 31) return true // 172.16.0.0/12 (Class B private)
    if (a === 192 && b === 168) return true // 192.168.0.0/16 (Class C private)
    if (a === 127) return true // loopback
    if (a === 169 && b === 254) return true // link-local
    return false
  }

  // build graph data with node budget and weighting
  const { nodes, edges, infoMsg, ipTotals, hostTotals } = useMemo(() => {
    if (!rows || rows.length === 0) return { nodes: [] as Node[], edges: [] as Edge[], infoMsg: 'No graph data' }

    // sort flows by total bytes, descending, so heavier edges are preferred
    const flows = [...rows].sort((a, b) => (b.bytes_in + b.bytes_out) - (a.bytes_in + a.bytes_out))

    const nodeMap = new Map<string, Node>()
    const ipTotalsMap = new Map<string, { bytes: number; packets: number }>()
    const hostTotalsMap = new Map<string, { bytes: number; packets: number }>()
    const edgesOut: Edge[] = []
    let edgeIndex = 0

    // helper to add/find nodes while keeping within budget
    const ensureNode = (id: string, label: string, type: NodeType, host?: string, iface?: string, hostId?: string): Node | null => {
      const exist = nodeMap.get(id)
      if (exist) return exist
      if (nodeMap.size + 1 > maxNodes) return null
      const n: Node = { id, label, type, host, iface, hostId, x: 0, y: 0 }
      nodeMap.set(id, n)
      return n
    }

    // First pass: decide which flows we can include based on node budget and accumulate totals
    const included: FlowRecord[] = []
    for (const r of flows) {
      const sipId = `ip:${r.sip}`
      const dipId = `ip:${r.dip}`
      const ifaceKey = r.iface ? `${r.host || 'host'}:${r.iface}` : 'unknown'
      const ifaceId = `iface:${ifaceKey}`
      const hostId = r.host ? `host:${r.host}` : undefined

      const needed = [
        nodeMap.has(sipId) ? 0 : 1,
        nodeMap.has(dipId) ? 0 : 1,
        nodeMap.has(ifaceId) ? 0 : 1,
        hostId ? (nodeMap.has(hostId) ? 0 : 1) : 0,
      ].reduce((a, b) => a + b, 0)

      if (nodeMap.size + needed > maxNodes && included.length > 0) break

      // add nodes within budget
      ensureNode(sipId, r.sip, 'ip')
      ensureNode(dipId, r.dip, 'ip')
      ensureNode(ifaceId, r.iface || '(iface)', 'iface', r.host, r.iface || undefined, r.host_id)
      if (hostId) ensureNode(hostId, r.host!, 'host', r.host, undefined, r.host_id)

      // accumulate totals
      const bytes = r.bytes_in + r.bytes_out
      const packets = r.packets_in + r.packets_out
      const sipTot = ipTotalsMap.get(r.sip) || { bytes: 0, packets: 0 }
      sipTot.bytes += bytes; sipTot.packets += packets
      ipTotalsMap.set(r.sip, sipTot)
      const dipTot = ipTotalsMap.get(r.dip) || { bytes: 0, packets: 0 }
      dipTot.bytes += bytes; dipTot.packets += packets
      ipTotalsMap.set(r.dip, dipTot)
      if (r.host) {
        const hTot = hostTotalsMap.get(r.host) || { bytes: 0, packets: 0 }
        hTot.bytes += bytes; hTot.packets += packets
        hostTotalsMap.set(r.host, hTot)
      }

      included.push(r)
    }

    // build edges: sip -> iface, iface -> dip (initial, per-flow)
    const maxBytes = Math.max(1, ...included.map(rr => rr.bytes_in + rr.bytes_out))
    for (const r of included) {
      const totalB = r.bytes_in + r.bytes_out
      const w = edgeWidth01(totalB / maxBytes)
      const color = r.bidirectional ? BLUE : RED
      const sipId2 = `ip:${r.sip}`
      const dipId2 = `ip:${r.dip}`
      const ifaceKey2 = r.iface ? `${r.host || 'host'}:${r.iface}` : 'unknown'
      const ifaceId2 = `iface:${ifaceKey2}`
      const packetsTotal = r.packets_in + r.packets_out
      const title = `${r.sip} → ${r.dip} (${r.iface || 'iface'} on ${r.host || 'host'})\nbytes in/out: ${r.bytes_in} / ${r.bytes_out}\npackets in/out: ${r.packets_in} / ${r.packets_out}`
      edgesOut.push({ id: `e:${edgeIndex++}:${sipId2}->${ifaceId2}`, from: sipId2, to: ifaceId2, color, width: w, title, sip: r.sip, dip: r.dip, bytesTotal: totalB, packetsTotal })
      edgesOut.push({ id: `e:${edgeIndex++}:${ifaceId2}->${dipId2}`, from: ifaceId2, to: dipId2, color, width: w, title, sip: r.sip, dip: r.dip, bytesTotal: totalB, packetsTotal })
    }

    // aggregate duplicate edges between same nodes (e.g., multiple flows sharing sip→iface)
    if (edgesOut.length) {
      const byPair = new Map<string, Edge & { count: number }>()
      for (const e of edgesOut) {
        const key = `${e.from}|${e.to}`
        const acc = byPair.get(key)
        if (acc) {
          acc.bytesTotal += e.bytesTotal
          acc.packetsTotal += e.packetsTotal
          acc.count += 1
          // if any flow is red (uni-directional), keep red
          if (e.color === RED) acc.color = RED
        } else {
          byPair.set(key, { ...e, count: 1 })
        }
      }
      // recompute widths based on aggregated totals
      const maxEdgeBytes = Math.max(1, ...Array.from(byPair.values()).map(x => x.bytesTotal))
      const aggregated: Edge[] = []
      let idx = 0
      for (const [key, v] of Array.from(byPair.entries())) {
        const w = edgeWidth01(v.bytesTotal / maxEdgeBytes)
        aggregated.push({ ...v, id: `ea:${idx++}:${key}`, width: w })
      }
      // replace edgesOut with aggregated list
      edgesOut.length = 0
      edgesOut.push(...aggregated)
    }

    // Lay out nodes: three columns (left IPs, middle interfaces (grouped by host), right IPs)
    // Determine sets
    const ipNodes: Node[] = []
    const ifaceNodesByHost = new Map<string, Node[]>()
    const hostNodes: Node[] = []
    for (const n of Array.from(nodeMap.values())) {
      if (n.type === 'ip') ipNodes.push(n)
      else if (n.type === 'iface') {
        const key = n.host || '(unknown)'
        const arr = ifaceNodesByHost.get(key) || []
        arr.push(n)
        ifaceNodesByHost.set(key, arr)
      } else if (n.type === 'host') hostNodes.push(n)
    }

    // helper volume for IP
    const vol = (n: Node) => ipTotalsMap.get(n.label)?.bytes || 0
    // sort ips by volume desc within their final groups later
    hostNodes.sort((a, b) => a.label.localeCompare(b.label))
    for (const arr of ifaceNodesByHost.values()) arr.sort((a, b) => (a.iface || '').localeCompare(b.iface || ''))

    const padding = 40
    const colX = {
      left: padding + 60,
      mid: width / 2,
      right: width - padding - 60,
    }

    // scatter IPs half on left (sources) and half on right (dests) based on roles in included flows
    const sourceSet = new Set(included.map(r => r.sip))
    const destSet = new Set(included.map(r => r.dip))
    const leftIPsAll = ipNodes.filter(n => sourceSet.has(n.label) && !destSet.has(n.label))
    const rightIPsAll = ipNodes.filter(n => destSet.has(n.label) && !sourceSet.has(n.label))
    const bothIPs = ipNodes.filter(n => sourceSet.has(n.label) && destSet.has(n.label))
    // split "both" across sides to balance
    const half = Math.ceil(bothIPs.length / 2)
    const leftIPs = [...leftIPsAll, ...bothIPs.slice(0, half)]
    const rightIPs = [...rightIPsAll, ...bothIPs.slice(half)]

    // group into private/public and sort by volume (desc) within each group
    const sortDesc = (arr: Node[]) => arr.sort((a, b) => vol(b) - vol(a) || a.label.localeCompare(b.label))
    const leftPriv = sortDesc(leftIPs.filter(n => isPrivateIP(n.label)))
    const leftPub = sortDesc(leftIPs.filter(n => !isPrivateIP(n.label)))
    const rightPriv = sortDesc(rightIPs.filter(n => isPrivateIP(n.label)))
    const rightPub = sortDesc(rightIPs.filter(n => !isPrivateIP(n.label)))

    // ensure at least MIN_IP_GAP between nodes; ensure GROUP_GAP between
    // private/public groups; prune from the tail if needed (lowest volume)
    const MIN_IP_GAP = 2 * IP_R + 8 // center-to-center spacing within a group (>= 8px edge gap)
    const GROUP_EDGE_GAP = 16 // minimum edge-to-edge gap between private and public groups
    const GROUP_CENTER_GAP = 2 * IP_R + GROUP_EDGE_GAP // convert to center-to-center spacing
    const distributeGrouped = (priv: Node[], pub: Node[], x: number, top: number, bottom: number) => {
      const span = Math.max(0, bottom - top)
      const havePriv = priv.length > 0
      const havePub = pub.length > 0
      if (!havePriv && !havePub) return
      if (havePriv && !havePub) {
        const maxFit = Math.max(1, Math.floor(span / MIN_IP_GAP) + 1)
        if (priv.length > maxFit) priv.length = maxFit
        if (priv.length === 1) { priv[0].x = x; priv[0].y = (top + bottom) / 2; return }
        const step = Math.max(MIN_IP_GAP, span / (priv.length - 1))
        const needed = step * (priv.length - 1)
        const start = top + Math.max(0, (span - needed) / 2)
        priv.forEach((n, i) => { n.x = x; n.y = start + i * step })
        return
      }
      if (!havePriv && havePub) {
        const maxFit = Math.max(1, Math.floor(span / MIN_IP_GAP) + 1)
        if (pub.length > maxFit) pub.length = maxFit
        if (pub.length === 1) { pub[0].x = x; pub[0].y = (top + bottom) / 2; return }
        const step = Math.max(MIN_IP_GAP, span / (pub.length - 1))
        const needed = step * (pub.length - 1)
        const start = top + Math.max(0, (span - needed) / 2)
        pub.forEach((n, i) => { n.x = x; n.y = start + i * step })
        return
      }
      // both groups present
      const canFit = () => {
        const gaps = (priv.length - 1) + (pub.length - 1)
        const minH = (gaps > 0 ? gaps * MIN_IP_GAP : 0) + GROUP_CENTER_GAP
        return minH <= span
      }
      while (!canFit()) {
        if (priv.length === 0 && pub.length === 0) break
        if (pub.length > priv.length) pub.pop()
        else if (priv.length > pub.length) priv.pop()
        else pub.pop() // prefer pruning public on ties
      }
      const gaps = (priv.length - 1) + (pub.length - 1)
      if (gaps <= 0) {
        const start = top + Math.max(0, (span - GROUP_CENTER_GAP) / 2)
        priv[0].x = x; priv[0].y = start
        pub[0].x = x; pub[0].y = start + GROUP_CENTER_GAP
        return
      }
      const step = Math.max(MIN_IP_GAP, (span - GROUP_CENTER_GAP) / gaps)
      const used = gaps * step + GROUP_CENTER_GAP
      const start = top + Math.max(0, (span - used) / 2)
      priv.forEach((n, i) => { n.x = x; n.y = start + i * step })
      const lastPrivY = start + (priv.length - 1) * step
      pub.forEach((n, i) => { n.x = x; n.y = lastPrivY + GROUP_CENTER_GAP + i * step })
    }

    const top = padding
    const bottom = height - padding
    distributeGrouped(leftPriv, leftPub, colX.left, top, bottom)
    distributeGrouped(rightPriv, rightPub, colX.right, top, bottom)

    // Remove IP nodes that couldn't be placed due to spacing constraints
    const keepIpIds = new Set<string>([...leftPriv, ...leftPub, ...rightPriv, ...rightPub].map(n => n.id))
    for (const n of ipNodes) {
      if (!keepIpIds.has(n.id)) nodeMap.delete(n.id)
    }
    // prune edges connected to removed nodes
    for (let i = edgesOut.length - 1; i >= 0; i--) {
      const e = edgesOut[i]
      if (!nodeMap.has(e.from) || !nodeMap.has(e.to)) edgesOut.splice(i, 1)
    }

    // Hosts vertically stacked in the middle; each host gets a bubble; interfaces distributed on bubble edge
    const hostOrder = hostNodes.length ? hostNodes : [{ id: 'host:(unknown)', label: '(unknown)', type: 'host', x: 0, y: 0 } as Node]
    // first pass: compute radius per host, factoring text width and iface count
    const MIN_HOST_GAP = 8
    hostOrder.forEach((h) => {
      const ifaces = ifaceNodesByHost.get(h.label) || []
      const N = Math.max(1, ifaces.length)
      // slightly smaller base radius due to smaller fonts
      let radius = Math.min(200, Math.max(80, 36 + N * 18))
      // ensure enough space for host label + totals centered
      const totals = hostTotalsMap.get(h.label) || { bytes: 0, packets: 0 }
      const bytesStr = formatBytesIEC(totals.bytes, totals.bytes >= 1024 * 1024 * 100 ? 0 : totals.bytes >= 1024 * 1024 * 10 ? 1 : 2)
        .replace('KiB', 'kB').replace('MiB', 'MB').replace('GiB', 'GB').replace('TiB', 'TB')
      const pktStr = (() => {
        const v = totals.packets
        if (v < 1000) return String(v)
        const units = ['K', 'M', 'B', 'T']
        let n = v, i = -1
        while (n >= 1000 && i < units.length - 1) { n /= 1000; i++ }
        return n.toFixed(n >= 100 ? 0 : n >= 10 ? 1 : 2) + ' ' + units[i]
      })()
      const label = h.label || 'host'
      const approx = (s: string, fs: number) => s.length * fs * 0.6
      // reduced font sizes: name 14, totals 12
      const maxTextWidth = Math.max(approx(label, 14), approx(`${bytesStr} / ${pktStr}`, 12))
      const pad = 20
      const neededR = (maxTextWidth + pad) / 2
      radius = Math.max(radius, neededR)
      h.radius = radius
      h.x = colX.mid
    })
    // second pass: vertically pack hosts with at least MIN_HOST_GAP between circles, centered block
    if (hostOrder.length > 0) {
      const available = bottom - top
      const n = hostOrder.length
      const totalCircles = hostOrder.reduce((s, h) => s + 2 * (h.radius || 100), 0)
      const blockHeight = totalCircles + (n - 1) * MIN_HOST_GAP
      const spacing = n > 1 ? MIN_HOST_GAP : 0
      // center the packed block vertically; don't stretch spacing
      const startTop = top + Math.max(0, (available - blockHeight) / 2)
      let y = startTop + (hostOrder[0].radius || 100)
      hostOrder[0].y = y
      for (let i = 1; i < n; i++) {
        const prev = hostOrder[i - 1]
        const cur = hostOrder[i]
        y += (prev.radius || 100) + spacing + (cur.radius || 100)
        cur.y = y
      }
      // place interfaces on the edge of each host circle
      hostOrder.forEach(h => {
        const ifaces = ifaceNodesByHost.get(h.label) || []
        const N = Math.max(1, ifaces.length)
        const r = h.radius || 100
        ifaces.forEach((n, i) => {
          const angle = (i / N) * Math.PI * 2
          n.x = (h.x) + Math.cos(angle) * r
          n.y = (h.y) + Math.sin(angle) * r
        })
      })
    }

    const infoMsg = rows.length > included.length
      ? `Graph limited to ${nodeMap.size} nodes across ${included.length} flows (of ${rows.length})`
      : undefined

    return { nodes: Array.from(nodeMap.values()), edges: edgesOut, infoMsg, ipTotals: ipTotalsMap, hostTotals: hostTotalsMap }
  }, [rows, maxNodes, width, height])

  if (loading && rows.length === 0) return <div className="p-4 text-sm text-gray-400">Loading graph…</div>
  if (!rows.length) return <div className="p-4 text-sm text-gray-400">No graph data</div>

  // helper to trim lines to circle edges
  // radii already declared above; keep local aliases for clarity
  // (values kept in sync)
  // const IP_R = 16
  // const IFACE_R = 12
  const getRadius = (n: Node | undefined) => !n ? 0 : (n.type === 'ip' ? IP_R : n.type === 'iface' ? IFACE_R : 0)
  const shorten = (a: Node | undefined, b: Node | undefined) => {
    if (!a || !b) return { x1: a?.x || 0, y1: a?.y || 0, x2: b?.x || 0, y2: b?.y || 0 }
    const dx = b.x - a.x
    const dy = b.y - a.y
    const len = Math.hypot(dx, dy) || 1
    const ux = dx / len
    const uy = dy / len
    const r1 = getRadius(a)
    const r2 = getRadius(b)
    return {
      x1: a.x + ux * r1,
      y1: a.y + uy * r1,
      x2: b.x - ux * r2,
      y2: b.y - uy * r2,
    }
  }

  // provide safe defaults if maps are undefined
  const safeHostTotals = hostTotals || new Map<string, { bytes: number; packets: number }>()
  const safeIpTotals = ipTotals || new Map<string, { bytes: number; packets: number }>()

  // filter edges to those touching the hovered interface (if any), otherwise hovered IP
  const edgesToRender: Edge[] = hoverIface
    ? edges.filter(e => e.from === hoverIface || e.to === hoverIface)
    : (hoverIP ? edges.filter(e => e.sip === hoverIP || e.dip === hoverIP) : [])
  const connectedNodeIds = new Set<string>()
  for (const e of edgesToRender) { connectedNodeIds.add(e.from); connectedNodeIds.add(e.to) }

  return (
    <div ref={containerRef} className="relative h-[70vh]">
      {infoMsg && <div className="absolute left-2 top-2 z-10 rounded bg-surface-100/70 px-2 py-1 text-[11px] text-gray-300 ring-1 ring-white/10">{infoMsg}</div>}
      <svg width="100%" height="100%" viewBox={`0 0 ${width} ${height}`}>

        {/* scale-to-fit group */}
        {(() => {
          // compute bounding box of all nodes, including label extents so nothing leaks out
          let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity
          const approx = (s: string, fs: number) => s.length * fs * 0.6
          const pktStr = (v: number) => {
            if (v < 1000) return String(v)
            const units = ['K', 'M', 'B', 'T']
            let n = v, i = -1
            while (n >= 1000 && i < units.length - 1) { n /= 1000; i++ }
            return n.toFixed(n >= 100 ? 0 : n >= 10 ? 1 : 2) + ' ' + units[i]
          }
          const measureNode = (n: Node) => {
            if (n.type === 'host') {
              const r = n.radius || 100
              return { minX: n.x - r, maxX: n.x + r, minY: n.y - r, maxY: n.y + r }
            }
            if (n.type === 'ip') {
              const totals = (safeIpTotals && safeIpTotals.get(n.label)) || { bytes: 0, packets: 0 }
              const totalStr = formatBytesIEC(totals.bytes, totals.bytes >= 1024 * 1024 * 100 ? 0 : totals.bytes >= 1024 * 1024 * 10 ? 1 : 2)
                .replace('KiB', 'kB').replace('MiB', 'MB').replace('GiB', 'GB').replace('TiB', 'TB')
              const pStr = pktStr(totals.packets)
              const textW = Math.max(approx(n.label, 12), approx(`${totalStr} / ${pStr}`, 12))
              // labels are side-aligned now: extend bbox to the left or right only
              // We can't know side here, so assume worst case to both sides
              const minX = n.x - IP_R - textW - 10
              const maxX = n.x + IP_R + textW + 10
              const minY = n.y - IP_R
              const maxY = n.y + IP_R
              return { minX, maxX, minY, maxY }
            }
            // iface: account for label above
            const label = n.iface || n.label
            const textW = approx(label, 11)
            const halfW = Math.max(IFACE_R, textW / 2)
            const minX = n.x - halfW
            const maxX = n.x + halfW
            const minY = n.y - (IFACE_R + 4 + 12) // circle + gap + text
            const maxY = n.y + IFACE_R
            return { minX, maxX, minY, maxY }
          }
          nodes.forEach(n => {
            const b = measureNode(n)
            minX = Math.min(minX, b.minX); maxX = Math.max(maxX, b.maxX)
            minY = Math.min(minY, b.minY); maxY = Math.max(maxY, b.maxY)
          })
          if (!isFinite(minX) || !isFinite(minY) || !isFinite(maxX) || !isFinite(maxY)) {
            minX = 0; minY = 0; maxX = width; maxY = height
          }
          const pad = 10
          const contentW = (maxX - minX) + pad
          const contentH = (maxY - minY) + pad
          const sx = (width - pad) / Math.max(1, contentW)
          const sy = (height - pad) / Math.max(1, contentH)
          const scale = Math.min(1, sx, sy)
          const tx = (width - (maxX - minX) * scale) / 2 - minX * scale
          const ty = (height - (maxY - minY) * scale) / 2 - minY * scale
          return (
            <g transform={`translate(${tx},${ty}) scale(${scale})`}>
              {/* host bubbles with centered label and totals */}
              {nodes.filter(n => n.type === 'host').map(h => {
                const r = h.radius || 100
                const totals = safeHostTotals.get(h.label) || { bytes: 0, packets: 0 }
                const bytesStr = formatBytesIEC(totals.bytes, totals.bytes >= 1024 * 1024 * 100 ? 0 : totals.bytes >= 1024 * 1024 * 10 ? 1 : 2)
                  .replace('KiB', 'kB').replace('MiB', 'MB').replace('GiB', 'GB').replace('TiB', 'TB')
                const pktStr = (() => {
                  const v = totals.packets
                  if (v < 1000) return String(v)
                  const units = ['K', 'M', 'B', 'T']
                  let n = v, i = -1
                  while (n >= 1000 && i < units.length - 1) { n /= 1000; i++ }
                  return n.toFixed(n >= 100 ? 0 : n >= 10 ? 1 : 2) + ' ' + units[i]
                })()
                return (
                  <g key={h.id} onClick={() => onHostClick?.(h.hostId || h.label)} style={{ cursor: 'pointer' }}>
                    <circle cx={h.x} cy={h.y} r={r} fill={HOST_FILL} stroke={HOST_STROKE} />
                    <text x={h.x} y={h.y - 3} textAnchor="middle" fontSize={14} fill="#ffffff" opacity={0.95}>{h.label || 'host'}</text>
                    <text x={h.x} y={h.y + 13} textAnchor="middle" fontSize={12} fill="#93c5fd" opacity={0.95}>{bytesStr} / {pktStr}</text>
                  </g>
                )
              })}

              {/* edges (only those connected to hovered IP). Draw a small midpoint arrow indicating direction (sip→dip) */}
              {edgesToRender.map(e => (
                <g key={e.id} pointerEvents="none">
                  {(() => {
                    const a = nodes.find(n => n.id === e.from)
                    const b = nodes.find(n => n.id === e.to)
                    const p = shorten(a, b)
                    // midpoint arrow geometry
                    const mx = (p.x1 + p.x2) / 2
                    const my = (p.y1 + p.y2) / 2
                    const dx = p.x2 - p.x1
                    const dy = p.y2 - p.y1
                    const len = Math.hypot(dx, dy) || 1
                    const ux = dx / len
                    const uy = dy / len
                    const px = -uy
                    const py = ux
                    // label text: totals, centered at midpoint, rotated parallel to edge
                    const textX = mx
                    const textY = my
                    const angleDeg = Math.atan2(dy, dx) * 180 / Math.PI
                    const bytesStr = formatBytesIEC(e.bytesTotal, e.bytesTotal >= 1024 * 1024 * 100 ? 0 : e.bytesTotal >= 1024 * 1024 * 10 ? 1 : 2)
                      .replace('KiB', 'kB').replace('MiB', 'MB').replace('GiB', 'GB').replace('TiB', 'TB')
                    const pktStr = (() => {
                      const v = e.packetsTotal
                      if (v < 1000) return String(v)
                      const units = ['K', 'M', 'B', 'T']
                      let n = v, i = -1
                      while (n >= 1000 && i < units.length - 1) { n /= 1000; i++ }
                      return n.toFixed(n >= 100 ? 0 : n >= 10 ? 1 : 2) + ' ' + units[i]
                    })()
                    // draw a small rounded rect background aligned to edge
                    const label = `${bytesStr} / ${pktStr}`
                    const fs = 11
                    const textW = Math.max(40, label.length * fs * 0.55)
                    const padX = 6, padY = 4, rxy = 4
                    const rectW = textW + padX * 2
                    const rectH = fs + padY * 2
                    const rectCx = textX - (rectW / 2)
                    const rectCy = textY - (rectH / 2)
                    // compute arrowhead positioned 16px to the right (forward) of the label box
                    const gap = 16
                    // stronger scaling with width; keep a small clearance from the label
                    const clearance = 4
                    const triBack = Math.min(gap - clearance, 2 + 2.2 * e.width)
                    const apexX = textX + ux * (rectW / 2 + gap)
                    const apexY = textY + uy * (rectW / 2 + gap)
                    const baseX = apexX - ux * triBack
                    const baseY = apexY - uy * triBack
                    // base width scales with edge width and triangle length for visibility
                    const halfBase = Math.max(e.width, triBack * 0.55)
                    const leftX = baseX + px * halfBase
                    const leftY = baseY + py * halfBase
                    const rightX = baseX - px * halfBase
                    const rightY = baseY - py * halfBase
                    const pts = `${leftX},${leftY} ${apexX},${apexY} ${rightX},${rightY}`
                    return (
                      <g>
                        <line x1={p.x1} y1={p.y1} x2={p.x2} y2={p.y2} stroke={e.color} strokeWidth={e.width} strokeLinecap="butt" opacity={0.9} />
                        <g transform={`rotate(${angleDeg}, ${textX}, ${textY})`}>
                          <rect x={rectCx} y={rectCy} width={rectW} height={rectH} rx={rxy} ry={rxy}
                            fill={panelBg} opacity={0.98} stroke={panelBg} strokeWidth={1} />
                          <text x={textX} y={textY} textAnchor="middle" dominantBaseline="middle" fontSize={fs} fill={e.color} opacity={0.98}>
                            {label}
                          </text>
                        </g>
                        <polygon points={pts} fill={e.color} opacity={0.98} stroke={e.color} strokeWidth={0.6} strokeLinejoin="round" />
                      </g>
                    )
                  })()}
                  <title>{e.title}</title>
                </g>
              ))}

              {/* interface nodes (circles) */}
              {nodes.filter(n => n.type === 'iface').map(n => (
                <g key={n.id}
                  onMouseEnter={() => setHoverIface(n.id)}
                  onMouseLeave={() => setHoverIface(null)}
                  onClick={() => onIfaceClick?.(n.hostId || n.host || '', n.iface || n.label)}
                  style={{ cursor: 'pointer' }}>
                  <circle cx={n.x} cy={n.y} r={IFACE_R} fill="#1f2937" stroke="#93c5fd" strokeWidth={2}
                    opacity={connectedNodeIds.size ? (connectedNodeIds.has(n.id) ? 1 : 0.2) : 1} />
                  <text x={n.x} y={n.y - (IFACE_R + 4)} textAnchor="middle" fontSize={11} fill="#93c5fd"
                    opacity={connectedNodeIds.size ? (connectedNodeIds.has(n.id) ? 1 : 0.2) : 1}>{n.iface || n.label}</text>
                </g>
              ))}

              {/* IP nodes (blue scheme). Label and totals (bytes / packets) beside node; align to side */}
              {nodes.filter(n => n.type === 'ip').map(n => {
                const dim = connectedNodeIds.size ? (connectedNodeIds.has(n.id) ? 1 : 0.4) : 1
                const totals = (safeIpTotals && safeIpTotals.get(n.label)) || { bytes: 0, packets: 0 }
                const totalStr = formatBytesIEC(totals.bytes, totals.bytes >= 1024 * 1024 * 100 ? 0 : totals.bytes >= 1024 * 1024 * 10 ? 1 : 2)
                  .replace('KiB', 'kB').replace('MiB', 'MB').replace('GiB', 'GB').replace('TiB', 'TB')
                const pktStr = (() => {
                  const v = totals.packets
                  if (v < 1000) return String(v)
                  const units = ['K', 'M', 'B', 'T']
                  let n2 = v, i = -1
                  while (n2 >= 1000 && i < units.length - 1) { n2 /= 1000; i++ }
                  return n2.toFixed(n2 >= 100 ? 0 : n2 >= 10 ? 1 : 2) + ' ' + units[i]
                })()
                // determine which side of the canvas this IP is on to align labels
                const isLeft = n.x < width / 2
                // Left column: label to the left of node, right-aligned
                // Right column: label to the right of node, left-aligned
                const labelX = isLeft ? (n.x - IP_R - 8) : (n.x + IP_R + 8)
                const anchor: 'start' | 'end' = isLeft ? 'end' : 'start'
                const isPriv = isPrivateIP(n.label)
                const fillColor = isPriv ? PRIVATE_FILL : PUBLIC_FILL
                const strokeColor = isPriv ? PRIVATE_STROKE : PUBLIC_STROKE
                return (
                  <g key={n.id}
                    onMouseEnter={() => setHoverIP(n.label)}
                    onMouseLeave={() => setHoverIP(null)}
                    onClick={() => onIpClick?.(n.label)}
                    style={{ cursor: 'pointer' }}>
                    <circle cx={n.x} cy={n.y} r={IP_R} fill={fillColor} opacity={0.22 * dim} stroke={strokeColor} strokeWidth={1.5} />
                    <text x={labelX} y={n.y - 3} textAnchor={anchor as any} fontSize={12} fill="#e5e7eb" opacity={dim}>{n.label}</text>
                    <text x={labelX} y={n.y + 12} textAnchor={anchor as any} fontSize={12} fill="#60a5fa" opacity={dim}>{totalStr} / {pktStr}</text>
                  </g>
                )
              })}
            </g>
          )
        })()}
      </svg>
    </div>
  )
}
