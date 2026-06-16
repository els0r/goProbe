import React, { useMemo, useRef, useEffect, useState } from 'react'
import { FlowRecord } from '../flows'
import { formatBytesIEC, humanBytes, humanPackets } from '../utils/format'
import { isPrivateIP } from '../utils/ipClassify'
import { buildGraph, layoutGraph, IP_R, IFACE_R } from './graph'
import type { Edge, PositionedNode } from './graph'

export interface GraphViewProps {
  rows: FlowRecord[]
  loading: boolean
  // cap total number of nodes (unique IPs + interfaces + hosts)
  maxNodes: number
  onIpClick?: (ip: string) => void
  onIfaceClick?: (host: string, iface: string) => void
  onHostClick?: (hostId: string) => void
}

// Label metrics (used for rect height and spacing guarantees)
const LABEL_FS = 11
const LABEL_PADY = 4

// Theme-aware graph palette. Colours live in --graph-* CSS custom properties
// (defined per theme in tokens.css) and are read here as whole colour strings,
// so a data-theme flip recolours the diagram without a reload.
interface GraphPalette {
  bg: string
  edge: string
  edgeHighlight: string
  publicFill: string
  publicStroke: string
  privateFill: string
  privateStroke: string
  ifaceFill: string
  ifaceStroke: string
  hostFill: string
  hostStroke: string
  label: string
  labelMuted: string
  labelAccent: string
}

function readGraphPalette(): GraphPalette {
  const s = getComputedStyle(document.documentElement)
  const v = (name: string) => s.getPropertyValue(name).trim()
  return {
    bg: v('--graph-bg'),
    edge: v('--graph-edge'),
    edgeHighlight: v('--graph-edge-highlight'),
    publicFill: v('--graph-node-public-fill'),
    publicStroke: v('--graph-node-public-stroke'),
    privateFill: v('--graph-node-private-fill'),
    privateStroke: v('--graph-node-private-stroke'),
    ifaceFill: v('--graph-node-iface-fill'),
    ifaceStroke: v('--graph-node-iface-stroke'),
    hostFill: v('--graph-host-fill'),
    hostStroke: v('--graph-host-stroke'),
    label: v('--graph-label'),
    labelMuted: v('--graph-label-muted'),
    labelAccent: v('--graph-label-accent'),
  }
}


// simple responsive container size hook
function useRect(ref: React.RefObject<HTMLElement | null>) {
  const [rect, setRect] = useState<{ width: number; height: number }>({
    width: 800,
    height: 500,
  })
  useEffect(() => {
    if (!ref.current) return
    const obs = new ResizeObserver((entries) => {
      const r = entries[0].contentRect
      setRect({
        width: Math.max(600, r.width),
        height: Math.max(400, r.height),
      })
    })
    obs.observe(ref.current)
    return () => obs.disconnect()
  }, [ref])
  return rect
}

export function GraphView({
  rows,
  loading,
  maxNodes,
  onIpClick,
  onIfaceClick,
  onHostClick,
}: GraphViewProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const { width, height } = useRect(containerRef)
  const [hoverIP, setHoverIP] = useState<string | null>(null)
  // Lazy initializer so first paint already has the correct (theme-resolved) palette.
  const [palette, setPalette] = useState<GraphPalette>(() => readGraphPalette())
  const [panelBg, setPanelBg] = useState<string>(() => readGraphPalette().bg)

  // detect nearest non-transparent background color to use as text outline fill
  const detectPanelBg = () => {
    let el: HTMLElement | null = containerRef.current
    let found: string | null = null
    try {
      while (el) {
        const bg = getComputedStyle(el).backgroundColor
        if (bg && bg !== 'rgba(0, 0, 0, 0)' && bg !== 'transparent') {
          found = bg
          break
        }
        el = el.parentElement as HTMLElement | null
      }
    } catch {
      // ignore
    }
    setPanelBg(found || readGraphPalette().bg)
  }

  // Re-read the palette (and the rendered panel background) whenever the theme
  // flips. applyTheme always rewrites data-theme on <html>, so a MutationObserver
  // on that attribute fires for both manual settings toggles and OS-driven changes.
  useEffect(() => {
    detectPanelBg()
    const obs = new MutationObserver(() => {
      setPalette(readGraphPalette())
      detectPanelBg()
    })
    obs.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme'],
    })
    return () => obs.disconnect()
  }, [])
  const [hoverIface, setHoverIface] = useState<string | null>(null)

  // The node-count-budget model: which Flows fit under maxNodes, plus their edges
  // and per-IP/per-host totals. Viewport-independent, so it recomputes only when
  // rows/maxNodes change — a resize or theme flip no longer re-runs aggregation.
  const model = useMemo(() => buildGraph(rows, { maxNodes }), [rows, maxNodes])
  const { ipTotals, hostTotals } = model

  // The spatial-budget layout: position the model's nodes for the current
  // viewport and prune any that can't fit. Recomputes only on model/size change.
  const layout = useMemo(() => layoutGraph(model, { width, height }), [model, width, height])
  const { nodes, edges } = layout

  // Behaviour-preserving message: still driven by the node-count budget only.
  // (layout.droppedForSpace is available too but deliberately not surfaced yet —
  // see docs/adr/0006-view-support-logic-framework-free-in-views.md.)
  const includedFlows = rows.length - model.droppedForCap
  const infoMsg =
    model.droppedForCap > 0
      ? `Graph limited to ${nodes.length} nodes across ${includedFlows} flows (of ${rows.length})`
      : undefined

  if (loading && rows.length === 0)
    return <div className="p-4 text-sm text-gray-400">Loading graph…</div>
  if (!rows.length) return <div className="p-4 text-sm text-gray-400">No graph data</div>

  // helper to trim lines to circle edges (IP_R/IFACE_R imported from ./graph so
  // render and layout agree on radii)
  const getRadius = (n: PositionedNode | undefined) =>
    !n ? 0 : n.type === 'ip' ? IP_R : n.type === 'iface' ? IFACE_R : 0
  const shorten = (a: PositionedNode | undefined, b: PositionedNode | undefined) => {
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
    ? edges.filter((e) => e.from === hoverIface || e.to === hoverIface)
    : hoverIP
      ? edges.filter(
        (e) => (e.sips && e.sips.includes(hoverIP)) || (e.dips && e.dips.includes(hoverIP))
      )
      : []
  const connectedNodeIds = new Set<string>()
  for (const e of edgesToRender) {
    connectedNodeIds.add(e.from)
    connectedNodeIds.add(e.to)
  }
  // detect reciprocal edges (A->B and B->A) so we can offset them apart for readability
  const edgeDirSet = new Set(edgesToRender.map((e) => `${e.from}|${e.to}`))
  // Build width map per unordered pair to decide which direction is thicker
  const pairWidths = new Map<string, { wMinMax: number; wMaxMin: number }>()
  for (const e of edgesToRender) {
    const minId = e.from < e.to ? e.from : e.to
    const maxId = e.from < e.to ? e.to : e.from
    const key = `${minId}|${maxId}`
    const entry = pairWidths.get(key) || { wMinMax: 0, wMaxMin: 0 }
    if (e.from === minId && e.to === maxId) entry.wMinMax = Math.max(entry.wMinMax, e.width)
    else entry.wMaxMin = Math.max(entry.wMaxMin, e.width)
    pairWidths.set(key, entry)
  }
  // z-ordering: draw edges towards IP first (under), then edges away from IP (over)
  const edgesToRenderSorted = edgesToRender.slice().sort((e1, e2) => {
    const nFrom1 = nodes.find((n) => n.id === e1.from)
    const nTo1 = nodes.find((n) => n.id === e1.to)
    const nFrom2 = nodes.find((n) => n.id === e2.from)
    const nTo2 = nodes.find((n) => n.id === e2.to)
    const pri = (from?: PositionedNode, to?: PositionedNode) => {
      if (from?.type === 'ip' && to?.type === 'iface') return 1 // away from IP (top)
      if (from?.type === 'iface' && to?.type === 'ip') return 0 // towards IP (bottom)
      return 0
    }
    const p1 = pri(nFrom1, nTo1)
    const p2 = pri(nFrom2, nTo2)
    if (p1 !== p2) return p1 - p2
    // Within same direction group, render uni-directional above bi-directional
    if (e1.dircat !== e2.dircat) {
      if (e1.dircat === 'bi' && e2.dircat === 'uni') return -1
      if (e1.dircat === 'uni' && e2.dircat === 'bi') return 1
    }
    // stable fallback: thicker last (slightly on top within same group)
    if (e1.width !== e2.width) return e1.width - e2.width
    return e1.id.localeCompare(e2.id)
  })
  // Group by direction (from|to) to separate multiple categories on same direction
  const dirGroups = new Map<string, Edge[]>()
  for (const e of edgesToRenderSorted) {
    const k = `${e.from}|${e.to}`
    const arr = dirGroups.get(k) || []
    arr.push(e)
    dirGroups.set(k, arr)
  }
  // Sort groups by width desc and build index lookup so thinner edges tuck closer to the base chord
  const dirIndexById = new Map<string, number>()
  const dirCountByKey = new Map<string, number>()
  for (const [k, arr] of dirGroups.entries()) {
    arr.sort((a, b) => b.width - a.width || a.id.localeCompare(b.id))
    dirCountByKey.set(k, arr.length)
    arr.forEach((edge, i) => dirIndexById.set(edge.id, i))
  }
  // Track which categories exist per direction (from|to)
  const dirCatsByKey = new Map<string, { bi: boolean; uni: boolean }>()
  for (const e of edgesToRenderSorted) {
    const k = `${e.from}|${e.to}`
    const entry = dirCatsByKey.get(k) || { bi: false, uni: false }
    if (e.dircat === 'bi') entry.bi = true
    else entry.uni = true
    dirCatsByKey.set(k, entry)
  }

  return (
    <div ref={containerRef} className="relative h-[70vh]">
      {infoMsg && (
        <div className="absolute left-2 top-2 z-10 rounded bg-surface-100/70 px-2 py-1 text-data-sm text-gray-300 ring-1 ring-line">
          {infoMsg}
        </div>
      )}
      <svg width="100%" height="100%" viewBox={`0 0 ${width} ${height}`}>
        {/* scale-to-fit group */}
        {(() => {
          // compute bounding box of all nodes, including label extents so nothing leaks out
          let minX = Infinity,
            minY = Infinity,
            maxX = -Infinity,
            maxY = -Infinity
          const approx = (s: string, fs: number) => s.length * fs * 0.6
          const pktStr = (v: number) => {
            if (v < 1000) return String(v)
            const units = ['K', 'M', 'B', 'T']
            let n = v,
              i = -1
            while (n >= 1000 && i < units.length - 1) {
              n /= 1000
              i++
            }
            return n.toFixed(n >= 100 ? 0 : n >= 10 ? 1 : 2) + ' ' + units[i]
          }
          const measureNode = (n: PositionedNode) => {
            if (n.type === 'host') {
              const r = n.radius || 100
              return {
                minX: n.x - r,
                maxX: n.x + r,
                minY: n.y - r,
                maxY: n.y + r,
              }
            }
            if (n.type === 'ip') {
              const totals = (safeIpTotals && safeIpTotals.get(n.label)) || {
                bytes: 0,
                packets: 0,
              }
              const totalStr = humanBytes(totals.bytes)
              const pStr = humanPackets(totals.packets)
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
          nodes.forEach((n) => {
            const b = measureNode(n)
            minX = Math.min(minX, b.minX)
            maxX = Math.max(maxX, b.maxX)
            minY = Math.min(minY, b.minY)
            maxY = Math.max(maxY, b.maxY)
          })
          if (!isFinite(minX) || !isFinite(minY) || !isFinite(maxX) || !isFinite(maxY)) {
            minX = 0
            minY = 0
            maxX = width
            maxY = height
          }
          const pad = 10
          const contentW = maxX - minX + pad
          const contentH = maxY - minY + pad
          const sx = (width - pad) / Math.max(1, contentW)
          const sy = (height - pad) / Math.max(1, contentH)
          const scale = Math.min(1, sx, sy)
          const tx = (width - (maxX - minX) * scale) / 2 - minX * scale
          const ty = (height - (maxY - minY) * scale) / 2 - minY * scale
          return (
            <g transform={`translate(${tx},${ty}) scale(${scale})`}>
              {/* host bubbles with centered label and totals */}
              {nodes
                .filter((n) => n.type === 'host')
                .map((h) => {
                  const r = h.radius || 100
                  const totals = safeHostTotals.get(h.label) || {
                    bytes: 0,
                    packets: 0,
                  }
                  const bytesStr = formatBytesIEC(
                    totals.bytes,
                    totals.bytes >= 1024 * 1024 * 100 ? 0 : totals.bytes >= 1024 * 1024 * 10 ? 1 : 2
                  )
                    .replace('KiB', 'kB')
                    .replace('MiB', 'MB')
                    .replace('GiB', 'GB')
                    .replace('TiB', 'TB')
                  const pktStr = (() => {
                    const v = totals.packets
                    if (v < 1000) return String(v)
                    const units = ['K', 'M', 'B', 'T']
                    let n = v,
                      i = -1
                    while (n >= 1000 && i < units.length - 1) {
                      n /= 1000
                      i++
                    }
                    return n.toFixed(n >= 100 ? 0 : n >= 10 ? 1 : 2) + ' ' + units[i]
                  })()
                  return (
                    <g
                      key={h.id}
                      onClick={() => onHostClick?.(h.hostId || h.label)}
                      style={{ cursor: 'pointer' }}
                    >
                      <circle
                        cx={h.x}
                        cy={h.y}
                        r={r}
                        fill={palette.hostFill}
                        stroke={palette.hostStroke}
                      />
                      <text
                        x={h.x}
                        y={h.y - 3}
                        textAnchor="middle"
                        fontSize={14}
                        fill={palette.label}
                        opacity={0.95}
                      >
                        {h.label || 'host'}
                      </text>
                      <text
                        x={h.x}
                        y={h.y + 13}
                        textAnchor="middle"
                        fontSize={12}
                        fill={palette.ifaceStroke}
                        opacity={0.95}
                      >
                        {bytesStr} / {pktStr}
                      </text>
                    </g>
                  )
                })}

              {/* edges (only those connected to hovered IP). Draw a small midpoint arrow indicating direction (sip→dip) */}
              {edgesToRenderSorted.map((e) => (
                <g key={e.id} pointerEvents="none">
                  {(() => {
                    const a = nodes.find((n) => n.id === e.from)
                    const b = nodes.find((n) => n.id === e.to)
                    // colour derived from the semantic directionality the model carries;
                    // the model itself stays palette-free (ADR-0006)
                    const color = e.dircat === 'bi' ? palette.edge : palette.edgeHighlight
                    const p = shorten(a, b)
                    // base vectors
                    const dx = p.x2 - p.x1
                    const dy = p.y2 - p.y1
                    const len = Math.hypot(dx, dy) || 1
                    const ux = dx / len
                    const uy = dy / len
                    // reciprocal detection -> draw as mirrored arcs using a quadratic Bezier
                    const hasReciprocal = edgeDirSet.has(`${e.to}|${e.from}`)
                    // Compute a stable base normal using the unordered pair (minId -> maxId)
                    const minId = e.from < e.to ? e.from : e.to
                    const maxId = e.from < e.to ? e.to : e.from
                    const nMin = nodes.find((n) => n.id === minId)
                    const nMax = nodes.find((n) => n.id === maxId)
                    const pBase = shorten(nMin, nMax)
                    const bdx = pBase.x2 - pBase.x1
                    const bdy = pBase.y2 - pBase.y1
                    const blen = Math.hypot(bdx, bdy) || 1
                    const bnx = -bdy / blen
                    const bny = bdx / blen
                    // pair max width for curvature scaling
                    const pairKey = `${minId}|${maxId}`
                    const pw = pairWidths.get(pairKey) || { wMinMax: 0, wMaxMin: 0 }
                    // control point around the base chord midpoint
                    const midX = (p.x1 + p.x2) / 2
                    const midY = (p.y1 + p.y2) / 2
                    // curvature magnitude: scale with edge width and segment length, clamped
                    const pairMaxW = Math.max(pw.wMinMax, pw.wMaxMin)
                    // Decide whether to curve: reciprocal present OR multiple edges share the same direction
                    const dirKey = `${e.from}|${e.to}`
                    const dirCount = dirCountByKey.get(dirKey) || 0
                    const shouldCurve = hasReciprocal || dirCount > 1
                    const baseCurv = shouldCurve
                      ? Math.max(12, Math.min(120, Math.min(len, 320) * 0.18 + 2.5 * pairMaxW))
                      : 0
                    // Category-based inner/outer: UNI = inner, BI = outer
                    // Enforce spacing >= label height + 4px between arcs (both across categories and within-category)
                    const cats = dirCatsByKey.get(dirKey) || { bi: false, uni: false }
                    const LABEL_HEIGHT = LABEL_FS + LABEL_PADY * 2
                    const MIN_ARC_GAP = LABEL_HEIGHT + 24
                    const dirArr = dirGroups.get(dirKey) || []
                    const sameCat = dirArr.filter((x) => x.dircat === e.dircat)
                    const idxInCat = sameCat.findIndex((x) => x.id === e.id)
                    // uni is inner (closer to chord); bi is outer by one MIN_ARC_GAP when both exist
                    const catBaseOffset =
                      cats.bi && cats.uni ? (e.dircat === 'bi' ? MIN_ARC_GAP : 0) : 0
                    // stagger additional edges within category by MIN_ARC_GAP each to avoid overlap
                    const catSpanOffset = Math.max(0, idxInCat) * MIN_ARC_GAP
                    const totalOffset = catBaseOffset + catSpanOffset
                    // Only apply curvature when needed; otherwise keep control on the chord to align label/arrow with straight edge
                    const curvMag = shouldCurve ? Math.max(8, baseCurv + totalOffset) : 0
                    // Side selection per direction (stable): min->max on +normal, max->min on -normal
                    const sgn = e.from === minId ? 1 : -1
                    const cx = midX + bnx * curvMag * sgn
                    const cy = midY + bny * curvMag * sgn
                    // label position at t=0.5 on the quadratic Bezier
                    const tLabel = 0.5
                    const oneMinusT = 1 - tLabel
                    const qx =
                      oneMinusT * oneMinusT * p.x1 +
                      2 * oneMinusT * tLabel * cx +
                      tLabel * tLabel * p.x2
                    const qy =
                      oneMinusT * oneMinusT * p.y1 +
                      2 * oneMinusT * tLabel * cy +
                      tLabel * tLabel * p.y2
                    // tangent (derivative) at t for rotation and arrow orientation
                    const dxdt = 2 * oneMinusT * (cx - p.x1) + 2 * tLabel * (p.x2 - cx)
                    const dydt = 2 * oneMinusT * (cy - p.y1) + 2 * tLabel * (p.y2 - cy)
                    const tLen = Math.hypot(dxdt, dydt) || 1
                    const tux = dxdt / tLen
                    const tuy = dydt / tLen
                    const tnx = -tuy
                    const tny = tux
                    // label center
                    const textX = qx
                    const textY = qy
                    const angleDegRaw = (Math.atan2(dydt, dxdt) * 180) / Math.PI
                    const angleDeg =
                      angleDegRaw > 90 || angleDegRaw < -90 ? angleDegRaw + 180 : angleDegRaw
                    const bytesStr = humanBytes(e.bytesTotal)
                    const pktStr = humanPackets(e.packetsTotal)
                    // draw a small rounded rect background aligned to edge
                    const label = `${bytesStr} / ${pktStr}`
                    const fs = 11
                    const textW = Math.max(40, label.length * fs * 0.55)
                    const padX = 6,
                      padY = 4,
                      rxy = 4
                    const rectW = textW + padX * 2
                    const rectH = fs + padY * 2
                    const rectCx = textX - rectW / 2
                    const rectCy = textY - rectH / 2
                    // Arrowhead: place apex directly on the curve, rotated to the tangent at that point
                    const tArrow = 0.7 // move arrow closer to destination along the curve
                    const oneMinusTA = 1 - tArrow
                    const ax =
                      oneMinusTA * oneMinusTA * p.x1 +
                      2 * oneMinusTA * tArrow * cx +
                      tArrow * tArrow * p.x2
                    const ay =
                      oneMinusTA * oneMinusTA * p.y1 +
                      2 * oneMinusTA * tArrow * cy +
                      tArrow * tArrow * p.y2
                    const dxdtA = 2 * oneMinusTA * (cx - p.x1) + 2 * tArrow * (p.x2 - cx)
                    const dydtA = 2 * oneMinusTA * (cy - p.y1) + 2 * tArrow * (p.y2 - cy)
                    const tLenA = Math.hypot(dxdtA, dydtA) || 1
                    const tuxA = dxdtA / tLenA
                    const tuyA = dydtA / tLenA
                    const tnxA = -tuyA
                    const tnyA = tuxA
                    // triangle dimensions scale with edge width
                    const triBack = 6 + 2.4 * e.width
                    const halfBase = Math.max(e.width, triBack * 0.55)
                    const apexX = ax
                    const apexY = ay
                    const baseX = apexX - tuxA * triBack
                    const baseY = apexY - tuyA * triBack
                    const leftX = baseX + tnxA * halfBase
                    const leftY = baseY + tnyA * halfBase
                    const rightX = baseX - tnxA * halfBase
                    const rightY = baseY - tnyA * halfBase
                    const pts = `${leftX},${leftY} ${apexX},${apexY} ${rightX},${rightY}`
                    return (
                      <g>
                        {shouldCurve ? (
                          <path
                            d={`M ${p.x1},${p.y1} Q ${cx},${cy} ${p.x2},${p.y2}`}
                            fill="none"
                            stroke={color}
                            strokeWidth={e.width}
                            strokeLinecap="butt"
                            opacity={0.9}
                          />
                        ) : (
                          <line
                            x1={p.x1}
                            y1={p.y1}
                            x2={p.x2}
                            y2={p.y2}
                            stroke={color}
                            strokeWidth={e.width}
                            strokeLinecap="butt"
                            opacity={0.9}
                          />
                        )}
                        <g transform={`rotate(${angleDeg}, ${textX}, ${textY})`}>
                          <rect
                            x={rectCx}
                            y={rectCy}
                            width={rectW}
                            height={rectH}
                            rx={rxy}
                            ry={rxy}
                            fill={panelBg}
                            fillOpacity={1}
                          />
                          <text
                            x={textX}
                            y={textY}
                            textAnchor="middle"
                            dominantBaseline="middle"
                            fontSize={fs}
                            fill={color}
                            opacity={1}
                          >
                            {label}
                          </text>
                        </g>
                        <polygon
                          points={pts}
                          fill={color}
                          opacity={0.98}
                          stroke={color}
                          strokeWidth={0.6}
                          strokeLinejoin="round"
                        />
                      </g>
                    )
                  })()}
                  <title>{e.title}</title>
                </g>
              ))}

              {/* interface nodes (circles) */}
              {nodes
                .filter((n) => n.type === 'iface')
                .map((n) => (
                  <g
                    key={n.id}
                    onMouseEnter={() => setHoverIface(n.id)}
                    onMouseLeave={() => setHoverIface(null)}
                    onClick={() => onIfaceClick?.(n.hostId || n.host || '', n.iface || n.label)}
                    style={{ cursor: 'pointer' }}
                  >
                    <circle
                      cx={n.x}
                      cy={n.y}
                      r={IFACE_R}
                      fill={palette.ifaceFill}
                      stroke={palette.ifaceStroke}
                      strokeWidth={2}
                      opacity={connectedNodeIds.size ? (connectedNodeIds.has(n.id) ? 1 : 0.2) : 1}
                    />
                    <text
                      x={n.x}
                      y={n.y - (IFACE_R + 4)}
                      textAnchor="middle"
                      fontSize={11}
                      fill={palette.ifaceStroke}
                      opacity={connectedNodeIds.size ? (connectedNodeIds.has(n.id) ? 1 : 0.2) : 1}
                    >
                      {n.iface || n.label}
                    </text>
                  </g>
                ))}

              {/* IP nodes (blue scheme). Label and totals (bytes / packets) beside node; align to side */}
              {nodes
                .filter((n) => n.type === 'ip')
                .map((n) => {
                  const dim = connectedNodeIds.size ? (connectedNodeIds.has(n.id) ? 1 : 0.4) : 1
                  const totals = (safeIpTotals && safeIpTotals.get(n.label)) || {
                    bytes: 0,
                    packets: 0,
                  }
                  const totalStr = formatBytesIEC(
                    totals.bytes,
                    totals.bytes >= 1024 * 1024 * 100 ? 0 : totals.bytes >= 1024 * 1024 * 10 ? 1 : 2
                  )
                    .replace('KiB', 'kB')
                    .replace('MiB', 'MB')
                    .replace('GiB', 'GB')
                    .replace('TiB', 'TB')
                  const pktStr = (() => {
                    const v = totals.packets
                    if (v < 1000) return String(v)
                    const units = ['K', 'M', 'B', 'T']
                    let n2 = v,
                      i = -1
                    while (n2 >= 1000 && i < units.length - 1) {
                      n2 /= 1000
                      i++
                    }
                    return n2.toFixed(n2 >= 100 ? 0 : n2 >= 10 ? 1 : 2) + ' ' + units[i]
                  })()
                  // determine which side of the canvas this IP is on to align labels
                  const isLeft = n.x < width / 2
                  // Left column: label to the left of node, right-aligned
                  // Right column: label to the right of node, left-aligned
                  const labelX = isLeft ? n.x - IP_R - 8 : n.x + IP_R + 8
                  const anchor: 'start' | 'end' = isLeft ? 'end' : 'start'
                  const isPriv = isPrivateIP(n.label)
                  const fillColor = isPriv ? palette.privateFill : palette.publicFill
                  const strokeColor = isPriv ? palette.privateStroke : palette.publicStroke
                  return (
                    <g
                      key={n.id}
                      onMouseEnter={() => setHoverIP(n.label)}
                      onMouseLeave={() => setHoverIP(null)}
                      onClick={() => onIpClick?.(n.label)}
                      style={{ cursor: 'pointer' }}
                    >
                      <circle
                        cx={n.x}
                        cy={n.y}
                        r={IP_R}
                        fill={fillColor}
                        opacity={0.22 * dim}
                        stroke={strokeColor}
                        strokeWidth={1.5}
                      />
                      <text
                        x={labelX}
                        y={n.y - 3}
                        textAnchor={anchor as any}
                        fontSize={12}
                        fill={palette.labelMuted}
                        opacity={dim}
                      >
                        {n.label}
                      </text>
                      <text
                        x={labelX}
                        y={n.y + 12}
                        textAnchor={anchor as any}
                        fontSize={12}
                        fill={palette.labelAccent}
                        opacity={dim}
                      >
                        {totalStr} / {pktStr}
                      </text>
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
