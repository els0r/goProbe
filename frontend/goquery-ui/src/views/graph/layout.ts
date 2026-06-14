// The flow-graph LAYOUT: the spatial-budget view of a Run's Flows.
//
// Pure and framework-free. Takes the geometry-free model and the current
// viewport and decides which nodes physically fit (with MIN_IP_GAP spacing) and
// where they sit — a three-column arrangement (source IPs | host bubbles with
// their interfaces | destination IPs). Nodes that don't fit are pruned and
// reported as droppedForSpace; the model's node-count budget is a separate
// concern. See docs/adr/0006-view-support-logic-framework-free-in-views.md.
import { isPrivateIP } from '../../utils/ipClassify'
import { humanBytes, humanPackets } from '../../utils/format'
import type { Edge, GraphModel, Node } from './model'

// node radii — exported so the renderer trims edges to the same circle sizes
export const IP_R = 16
export const IFACE_R = 12

// A model Node with its computed position. Geometry exists only after layout, so
// it lives on this type, never on Node.
export interface PositionedNode extends Node {
  x: number
  y: number
  radius?: number // host bubbles only
}

export interface GraphLayout {
  nodes: PositionedNode[]
  edges: Edge[]
  // Nodes pruned because they could not fit the viewport with minimum spacing.
  droppedForSpace: number
}

export interface LayoutOptions {
  width: number
  height: number
}

// layoutGraph positions the model's nodes for the given viewport and prunes any
// that cannot fit. Total: it never throws and never mutates the model.
export function layoutGraph(model: GraphModel, { width, height }: LayoutOptions): GraphLayout {
  // Position on fresh PositionedNode copies so the (geometry-free) model is left
  // untouched and re-layout on resize starts from the full candidate set.
  const nodeMap = new Map<string, PositionedNode>(
    model.nodes.map((n) => [n.id, { ...n, x: 0, y: 0 }])
  )
  const edgesOut: Edge[] = model.edges.slice()

  // Split nodes by type; group interfaces under their host.
  const ipNodes: PositionedNode[] = []
  const ifaceNodesByHost = new Map<string, PositionedNode[]>()
  const hostNodes: PositionedNode[] = []
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
  const vol = (n: PositionedNode) => model.ipTotals.get(n.label)?.bytes || 0
  hostNodes.sort((a, b) => a.label.localeCompare(b.label))
  for (const arr of ifaceNodesByHost.values())
    arr.sort((a, b) => (a.iface || '').localeCompare(b.iface || ''))

  const padding = 40
  const colX = {
    left: padding + 60,
    mid: width / 2,
    right: width - padding - 60,
  }

  // scatter IPs onto left (sources) / right (dests) by the roles they play in the
  // model's edges: an IP in some edge's sips is a source, in some edge's dips a dest
  const sourceSet = new Set<string>()
  const destSet = new Set<string>()
  for (const e of edgesOut) {
    for (const s of e.sips) sourceSet.add(s)
    for (const d of e.dips) destSet.add(d)
  }
  const leftIPsAll = ipNodes.filter((n) => sourceSet.has(n.label) && !destSet.has(n.label))
  const rightIPsAll = ipNodes.filter((n) => destSet.has(n.label) && !sourceSet.has(n.label))
  const bothIPs = ipNodes.filter((n) => sourceSet.has(n.label) && destSet.has(n.label))
  // split "both" across sides to balance
  const half = Math.ceil(bothIPs.length / 2)
  const leftIPs = [...leftIPsAll, ...bothIPs.slice(0, half)]
  const rightIPs = [...rightIPsAll, ...bothIPs.slice(half)]

  // group into private/public and sort by volume (desc) within each group
  const sortDesc = (arr: PositionedNode[]) =>
    arr.sort((a, b) => vol(b) - vol(a) || a.label.localeCompare(b.label))
  const leftPriv = sortDesc(leftIPs.filter((n) => isPrivateIP(n.label)))
  const leftPub = sortDesc(leftIPs.filter((n) => !isPrivateIP(n.label)))
  const rightPriv = sortDesc(rightIPs.filter((n) => isPrivateIP(n.label)))
  const rightPub = sortDesc(rightIPs.filter((n) => !isPrivateIP(n.label)))

  // ensure at least MIN_IP_GAP between nodes; ensure GROUP_GAP between
  // private/public groups; prune from the tail if needed (lowest volume)
  const MIN_IP_GAP = 2 * IP_R + 8 // center-to-center spacing within a group (>= 8px edge gap)
  const GROUP_EDGE_GAP = 16 // minimum edge-to-edge gap between private and public groups
  const GROUP_CENTER_GAP = 2 * IP_R + GROUP_EDGE_GAP // convert to center-to-center spacing
  const distributeGrouped = (
    priv: PositionedNode[],
    pub: PositionedNode[],
    x: number,
    top: number,
    bottom: number
  ) => {
    const span = Math.max(0, bottom - top)
    const havePriv = priv.length > 0
    const havePub = pub.length > 0
    if (!havePriv && !havePub) return
    if (havePriv && !havePub) {
      const maxFit = Math.max(1, Math.floor(span / MIN_IP_GAP) + 1)
      if (priv.length > maxFit) priv.length = maxFit
      if (priv.length === 1) {
        priv[0].x = x
        priv[0].y = (top + bottom) / 2
        return
      }
      const step = Math.max(MIN_IP_GAP, span / (priv.length - 1))
      const needed = step * (priv.length - 1)
      const start = top + Math.max(0, (span - needed) / 2)
      priv.forEach((n, i) => {
        n.x = x
        n.y = start + i * step
      })
      return
    }
    if (!havePriv && havePub) {
      const maxFit = Math.max(1, Math.floor(span / MIN_IP_GAP) + 1)
      if (pub.length > maxFit) pub.length = maxFit
      if (pub.length === 1) {
        pub[0].x = x
        pub[0].y = (top + bottom) / 2
        return
      }
      const step = Math.max(MIN_IP_GAP, span / (pub.length - 1))
      const needed = step * (pub.length - 1)
      const start = top + Math.max(0, (span - needed) / 2)
      pub.forEach((n, i) => {
        n.x = x
        n.y = start + i * step
      })
      return
    }
    // both groups present
    const canFit = () => {
      const gaps = priv.length - 1 + (pub.length - 1)
      const minH = (gaps > 0 ? gaps * MIN_IP_GAP : 0) + GROUP_CENTER_GAP
      return minH <= span
    }
    while (!canFit()) {
      if (priv.length === 0 && pub.length === 0) break
      if (pub.length > priv.length) pub.pop()
      else if (priv.length > pub.length) priv.pop()
      else pub.pop() // prefer pruning public on ties
    }
    const gaps = priv.length - 1 + (pub.length - 1)
    if (gaps <= 0) {
      const start = top + Math.max(0, (span - GROUP_CENTER_GAP) / 2)
      priv[0].x = x
      priv[0].y = start
      pub[0].x = x
      pub[0].y = start + GROUP_CENTER_GAP
      return
    }
    const step = Math.max(MIN_IP_GAP, (span - GROUP_CENTER_GAP) / gaps)
    const used = gaps * step + GROUP_CENTER_GAP
    const start = top + Math.max(0, (span - used) / 2)
    priv.forEach((n, i) => {
      n.x = x
      n.y = start + i * step
    })
    const lastPrivY = start + (priv.length - 1) * step
    pub.forEach((n, i) => {
      n.x = x
      n.y = lastPrivY + GROUP_CENTER_GAP + i * step
    })
  }

  const top = padding
  const bottom = height - padding
  distributeGrouped(leftPriv, leftPub, colX.left, top, bottom)
  distributeGrouped(rightPriv, rightPub, colX.right, top, bottom)

  // Remove IP nodes that couldn't be placed due to spacing constraints
  const keepIpIds = new Set<string>(
    [...leftPriv, ...leftPub, ...rightPriv, ...rightPub].map((n) => n.id)
  )
  const droppedForSpace = ipNodes.length - keepIpIds.size
  for (const n of ipNodes) {
    if (!keepIpIds.has(n.id)) nodeMap.delete(n.id)
  }
  // prune edges connected to removed nodes
  for (let i = edgesOut.length - 1; i >= 0; i--) {
    const e = edgesOut[i]
    if (!nodeMap.has(e.from) || !nodeMap.has(e.to)) edgesOut.splice(i, 1)
  }

  // Hosts vertically stacked in the middle; each host gets a bubble; interfaces distributed on bubble edge
  const hostOrder: PositionedNode[] = hostNodes.length
    ? hostNodes
    : [
        {
          id: 'host:(unknown)',
          label: '(unknown)',
          type: 'host',
          x: 0,
          y: 0,
        } as PositionedNode,
      ]
  // first pass: compute radius per host, factoring text width and iface count
  const MIN_HOST_GAP = 8
  hostOrder.forEach((h) => {
    const ifaces = ifaceNodesByHost.get(h.label) || []
    const N = Math.max(1, ifaces.length)
    // slightly smaller base radius due to smaller fonts
    let radius = Math.min(200, Math.max(80, 36 + N * 18))
    // ensure enough space for host label + totals centered
    const totals = model.hostTotals.get(h.label) || { bytes: 0, packets: 0 }
    const bytesStr = humanBytes(totals.bytes)
    const pktStr = humanPackets(totals.packets)
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
    hostOrder.forEach((h) => {
      const ifaces = ifaceNodesByHost.get(h.label) || []
      const N = Math.max(1, ifaces.length)
      const r = h.radius || 100
      ifaces.forEach((n, i) => {
        const angle = (i / N) * Math.PI * 2
        n.x = h.x + Math.cos(angle) * r
        n.y = h.y + Math.sin(angle) * r
      })
    })
  }

  return {
    nodes: Array.from(nodeMap.values()),
    edges: edgesOut,
    droppedForSpace,
  }
}
