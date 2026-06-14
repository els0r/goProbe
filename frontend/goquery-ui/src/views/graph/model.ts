// The flow-graph MODEL: the node-count-budget view of a Run's Flows.
//
// Pure, framework-free, viewport-independent — it depends only on (rows,
// maxNodes) and never throws. The over-budget soft-failure is reported as a
// count (droppedForCap), not a string: the view owns the user-facing message.
// Geometry (x/y/radius) and colour are NOT decided here — positions come from
// layoutGraph (./layout) and colour is derived at render from `dircat`.
//
// This is view-support logic, framework-free and tested, living under views/ —
// see docs/adr/0006-view-support-logic-framework-free-in-views.md.
import type { FlowRecord } from '../../flows'

export type NodeType = 'ip' | 'iface' | 'host'

export interface Node {
  id: string
  label: string
  type: NodeType
  // grouping keys
  host?: string
  iface?: string
  hostId?: string
  // No geometry here: positions live on PositionedNode (./layout), assigned by
  // layoutGraph. The model is viewport-independent.
}

export interface Edge {
  id: string
  from: string
  to: string
  width: number
  title: string
  sip: string
  dip: string
  bytesTotal: number
  packetsTotal: number
  // Source/destination IPs touched by this (possibly aggregated) edge.
  sips: string[]
  dips: string[]
  // Directionality category: 'bi' bidirectional, 'uni' unidirectional. Colour is
  // derived from this at render time, so the model carries no palette.
  dircat: 'bi' | 'uni'
}

export interface Totals {
  bytes: number
  packets: number
}

export interface GraphModel {
  nodes: Node[]
  edges: Edge[]
  ipTotals: Map<string, Totals>
  hostTotals: Map<string, Totals>
  // Number of Flows left out because the node-count budget (maxNodes) filled up.
  droppedForCap: number
}

export interface BuildGraphOptions {
  maxNodes: number
}

// map a 0..1 value to stroke width 1..8px in 12.5% buckets
export function edgeWidth01(v01: number): number {
  if (!isFinite(v01) || v01 <= 0) return 1
  const bin = Math.min(7, Math.floor(v01 * 8)) // 0..7
  return 1 + bin
}

// buildGraph turns a Run's Flows into the graph's nodes/edges plus per-IP and
// per-host totals, admitting Flows (heaviest first) only while the node budget
// holds. Total: an empty rows array yields an empty model, never an error.
export function buildGraph(rows: FlowRecord[], { maxNodes }: BuildGraphOptions): GraphModel {
  if (!rows || rows.length === 0) {
    return {
      nodes: [],
      edges: [],
      ipTotals: new Map(),
      hostTotals: new Map(),
      droppedForCap: 0,
    }
  }

  // sort flows by total bytes, descending, so heavier edges are preferred
  const flows = [...rows].sort((a, b) => b.bytes_total - a.bytes_total)

  const nodeMap = new Map<string, Node>()
  const ipTotalsMap = new Map<string, Totals>()
  const hostTotalsMap = new Map<string, Totals>()
  const edgesOut: Edge[] = []
  let edgeIndex = 0

  // add/find nodes while keeping within budget
  const ensureNode = (
    id: string,
    label: string,
    type: NodeType,
    host?: string,
    iface?: string,
    hostId?: string
  ): Node | null => {
    const exist = nodeMap.get(id)
    if (exist) return exist
    if (nodeMap.size + 1 > maxNodes) return null
    const n: Node = { id, label, type, host, iface, hostId }
    nodeMap.set(id, n)
    return n
  }

  // First pass: decide which flows fit the node budget and accumulate totals
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
    const bytes = r.bytes_total
    const packets = r.packets_total
    const sipTot = ipTotalsMap.get(r.sip) || { bytes: 0, packets: 0 }
    sipTot.bytes += bytes
    sipTot.packets += packets
    ipTotalsMap.set(r.sip, sipTot)
    const dipTot = ipTotalsMap.get(r.dip) || { bytes: 0, packets: 0 }
    dipTot.bytes += bytes
    dipTot.packets += packets
    ipTotalsMap.set(r.dip, dipTot)
    if (r.host) {
      const hTot = hostTotalsMap.get(r.host) || { bytes: 0, packets: 0 }
      hTot.bytes += bytes
      hTot.packets += packets
      hostTotalsMap.set(r.host, hTot)
    }

    included.push(r)
  }

  // build edges: sip -> iface, iface -> dip (initial, per-flow)
  const maxBytes = Math.max(1, ...included.map((rr) => rr.bytes_total))
  for (const r of included) {
    const totalB = r.bytes_total
    const w = edgeWidth01(totalB / maxBytes)
    const dircat: 'bi' | 'uni' = r.bidirectional ? 'bi' : 'uni'
    const sipId2 = `ip:${r.sip}`
    const dipId2 = `ip:${r.dip}`
    const ifaceKey2 = r.iface ? `${r.host || 'host'}:${r.iface}` : 'unknown'
    const ifaceId2 = `iface:${ifaceKey2}`
    const packetsTotal = r.packets_total
    const title = `${r.sip} → ${r.dip} (${r.iface || 'iface'} on ${r.host || 'host'})\nbytes in/out: ${r.bytes_in} / ${r.bytes_out}\npackets in/out: ${r.packets_in} / ${r.packets_out}`
    edgesOut.push({
      id: `e:${edgeIndex++}:${sipId2}->${ifaceId2}`,
      from: sipId2,
      to: ifaceId2,
      width: w,
      title,
      sip: r.sip,
      dip: r.dip,
      bytesTotal: totalB,
      packetsTotal,
      sips: [r.sip],
      dips: [r.dip],
      dircat,
    })
    edgesOut.push({
      id: `e:${edgeIndex++}:${ifaceId2}->${dipId2}`,
      from: ifaceId2,
      to: dipId2,
      width: w,
      title,
      sip: r.sip,
      dip: r.dip,
      bytesTotal: totalB,
      packetsTotal,
      sips: [r.sip],
      dips: [r.dip],
      dircat,
    })
  }

  // aggregate duplicate edges between same nodes (e.g. multiple flows sharing
  // sip→iface). bi/uni never merge — the dircat is part of the bucket key.
  if (edgesOut.length) {
    const byPair = new Map<string, Edge & { count: number }>()
    for (const e of edgesOut) {
      const key = `${e.from}|${e.to}|${e.dircat}`
      const acc = byPair.get(key)
      if (acc) {
        acc.bytesTotal += e.bytesTotal
        acc.packetsTotal += e.packetsTotal
        acc.count += 1
        // merge membership (unique)
        const addUniq = (arr: string[], v: string) => {
          if (!arr.includes(v)) arr.push(v)
        }
        for (const s of e.sips) addUniq(acc.sips, s)
        for (const d of e.dips) addUniq(acc.dips, d)
      } else {
        byPair.set(key, { ...e, count: 1 })
      }
    }
    // recompute widths based on aggregated totals
    const maxEdgeBytes = Math.max(1, ...Array.from(byPair.values()).map((x) => x.bytesTotal))
    const aggregated: Edge[] = []
    let idx = 0
    for (const [key, v] of Array.from(byPair.entries())) {
      const w = edgeWidth01(v.bytesTotal / maxEdgeBytes)
      aggregated.push({ ...v, id: `ea:${idx++}:${key}`, width: w })
    }
    edgesOut.length = 0
    edgesOut.push(...aggregated)
  }

  return {
    nodes: Array.from(nodeMap.values()),
    edges: edgesOut,
    ipTotals: ipTotalsMap,
    hostTotals: hostTotalsMap,
    droppedForCap: rows.length - included.length,
  }
}
