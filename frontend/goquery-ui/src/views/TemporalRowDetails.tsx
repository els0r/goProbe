import React from 'react'
import { FlowRecord } from '../flows'
import { SummarySchema } from '../api/domain'
import { DrillBucket, DrillSnapshot, TemporalMeta } from '../query/detailRunner'
import { DetailsCard } from '../components/DetailsCard'
import { ServiceDetailsCard } from '../components/ServiceDetailsCard'
import { AttributeOption } from '../components/AttributesSelect'
import { humanBytes, humanPackets } from '../utils/format'
import { renderError } from '../utils/renderError'
import { renderProto } from '../utils/proto'
import { extractOffset, formatInOffset, buildIntervalLabel } from '../utils/temporal'
import { formatTimestamp } from '../utils/timeFormat'
import { groupByService } from '../flows'
import { InOutBar } from '../components/InOutBar'
import { inOutScaleMax } from '../flows'

const ATTR_OPTIONS: AttributeOption[] = [
    { label: 'Source IP', value: 'sip' },
    { label: 'Destination IP', value: 'dip' },
    { label: 'Port', value: 'dport' },
    { label: 'IP Protocol', value: 'proto' },
]

export interface TemporalRowDetailsProps {
    rows: FlowRecord[]
    loading: boolean
    error?: unknown
    summary?: SummarySchema
    colSpan: number
    /** ISO string for the query --first parameter */
    queryFirst?: string
    /** ISO string for the query --last parameter */
    queryLast?: string
    /** Row identity of the open Detail Run */
    meta?: TemporalMeta
    /** Attributes used in the original query */
    attrsShown?: string[]
    /** Render in/out as diverging gauges instead of dense numeric columns */
    visualInOutBars?: boolean
    /** Flank the diverging gauges with the in/out figures that shaped them */
    showDirectionValues?: boolean
    /** Drill-down slot of the DetailRunner snapshot */
    drill?: DrillSnapshot | null
    onDrill?: (bucket: DrillBucket, attrs: { values: string[]; all: boolean }) => void
    onCloseDrill?: () => void
}

const MAX_TILES = 144

interface Bucket {
    start: number
    end: number
    bytesIn: number
    bytesOut: number
    rowIndices: number[]
}

export function TemporalRowDetails({
    rows,
    loading,
    error,
    summary,
    colSpan,
    queryFirst,
    queryLast,
    meta,
    attrsShown,
    visualInOutBars = false,
    showDirectionValues = false,
    drill,
    onDrill,
    onCloseDrill,
}: TemporalRowDetailsProps) {
    // Selection is a contiguous tile range. anchor = drag start, focus = drag end.
    // A single-tile selection has anchor === focus; null = nothing selected.
    const [selection, setSelection] = React.useState<{ anchor: number; focus: number } | null>(null)
    const rowRef = React.useRef<HTMLDivElement | null>(null)
    const dragging = React.useRef(false)
    // Render-visible mirror of `dragging`: while true we show a lightweight readout
    // instead of the full DetailsCard, which would be costly to re-render per move.
    const [isDragging, setIsDragging] = React.useState(false)
    const press = React.useRef<{ idx: number; moved: boolean; wasSingle: boolean }>({
        idx: 0,
        moved: false,
        wasSingle: false,
    })

    // Parent totals across all temporal rows (used for share bars in child cards)
    const rowTotals = React.useMemo(() => {
        let b = 0, p = 0
        for (const r of rows) {
            b += r.bytes_total
            p += r.packets_total
        }
        return { bytes: b, packets: p }
    }, [rows])

    // Drill-down state
    const complementAttrs = React.useMemo(() => {
        const shown = new Set(attrsShown || [])
        return ATTR_OPTIONS.map((o) => o.value).filter((v) => !shown.has(v))
    }, [attrsShown])

    // Executed Drill-down state lives in the DetailRunner's drill slot; the
    // selection auto-runs it (see the effect below) — there is no draft UI.
    const drillLoading = drill?.phase === 'loading'
    const drillError = drill?.phase === 'error' ? drill.error : null

    const times = rows.map((r) => r.interval_end || '').filter(Boolean)

    const resolutionNs = (summary as any)?.timings?.resolution
    const durMs: number = Number.isFinite(resolutionNs)
        ? Math.max(1, Math.round(Number(resolutionNs) / 1_000_000))
        : 5 * 60 * 1000

    // queryFirst/queryLast carry the raw query params, which may be relative
    // specs (e.g. "-10m") that Date.parse can't read. Use them only when they
    // parse to a finite timestamp; otherwise fall back to the backend's resolved
    // absolute bounds. A NaN bound here would collapse tileCount to NaN and crash
    // the bucket loop on arr[NaN].
    const parseFiniteMs = (val?: string): number | null => {
        if (!val) return null
        const ms = Date.parse(val)
        return Number.isFinite(ms) ? ms : null
    }

    const firstMs =
        parseFiniteMs(queryFirst) ??
        parseFiniteMs((summary as any)?.time_first) ??
        (times.length
            ? Math.min(...times.map((t) => Date.parse(t)))
            : Date.now() - 12 * 60 * 60 * 1000)
    const lastMs =
        parseFiniteMs(queryLast) ??
        parseFiniteMs((summary as any)?.time_last) ??
        (times.length ? Math.max(...times.map((t) => Date.parse(t))) : Date.now())

    const spanMs = Math.max(1, lastMs - firstMs)
    const rawTileCount = Math.max(1, Math.ceil(spanMs / durMs))
    const tileCount = Math.min(rawTileCount, MAX_TILES)
    const tileMs = spanMs / tileCount

    const offsetInfo = extractOffset(
        queryLast || (summary as any)?.time_last || times[0] || new Date().toISOString()
    )

    const buckets: Bucket[] = React.useMemo(() => {
        const arr: Bucket[] = Array.from({ length: tileCount }, (_, i) => {
            const start = firstMs + i * tileMs
            return { start, end: start + tileMs, bytesIn: 0, bytesOut: 0, rowIndices: [] }
        })
        rows.forEach((r, i) => {
            const e = r.interval_end ? Date.parse(r.interval_end) : NaN
            if (!Number.isFinite(e)) return
            if (e <= firstMs || e > lastMs) return
            let idx = Math.floor((e - firstMs) / tileMs)
            if (idx < 0) idx = 0
            if (idx >= tileCount) idx = tileCount - 1
            arr[idx].bytesIn += r.bytes_in || 0
            arr[idx].bytesOut += r.bytes_out || 0
            arr[idx].rowIndices.push(i)
        })
        return arr
    }, [rows, firstMs, lastMs, tileCount, tileMs])

    const maxBucket = buckets.reduce((m, b) => Math.max(m, b.bytesIn + b.bytesOut), 0)

    // Committed range, normalized + clamped to the current tile grid.
    const range = React.useMemo(() => {
        if (!selection) return null
        const lo = Math.max(0, Math.min(selection.anchor, selection.focus))
        const hi = Math.min(tileCount - 1, Math.max(selection.anchor, selection.focus))
        return { lo, hi }
    }, [selection, tileCount])

    // A finalized tile selection auto-runs the Drill-down on the complement
    // attributes (those not in the original query). Any change to the committed
    // range supersedes the prior run; clearing the selection, dragging, or having
    // nothing left to break down tears it down. The DetailRunner's generation
    // counter aborts in-flight fetches on supersession.
    React.useEffect(() => {
        const lo = range ? buckets[range.lo] : undefined
        const hi = range ? buckets[range.hi] : undefined
        const matchesDrill =
            !!drill && !!lo && !!hi &&
            drill.bucket.startMs === lo.start && drill.bucket.endMs === hi.end
        if (!range || isDragging || !lo || !hi || complementAttrs.length === 0) {
            if (drill && !matchesDrill) onCloseDrill?.()
            return
        }
        if (matchesDrill) return
        onDrill?.({ startMs: lo.start, endMs: hi.end }, { values: complementAttrs, all: false })
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [range, drill, buckets, isDragging, complementAttrs])

    // Color: blue for bidirectional, red for uni-directional
    const tileColorClass = (b: Bucket) => {
        const total = b.bytesIn + b.bytesOut
        const share = maxBucket > 0 ? total / maxBucket : 0
        if (!Number.isFinite(share) || share <= 0) return 'bg-surface-100 ring-line'
        const uni = b.bytesIn === 0 || b.bytesOut === 0
        if (uni) {
            if (share < 0.2) return 'bg-red-500/20 ring-red-500/30'
            if (share < 0.4) return 'bg-red-500/35 ring-red-500/40'
            if (share < 0.6) return 'bg-red-500/55 ring-red-500/50'
            if (share < 0.8) return 'bg-red-500/70 ring-red-500/60'
            return 'bg-red-500/90 ring-red-500/70'
        }
        if (share < 0.2) return 'bg-primary-500/20 ring-primary-500/30'
        if (share < 0.4) return 'bg-primary-500/35 ring-primary-500/40'
        if (share < 0.6) return 'bg-primary-500/55 ring-primary-500/50'
        if (share < 0.8) return 'bg-primary-500/70 ring-primary-500/60'
        return 'bg-primary-500/90 ring-primary-500/70'
    }

    const tileTitle = (b: Bucket) => {
        const left = formatInOffset(b.start, offsetInfo.minutes, true, true, offsetInfo.suffix)
        const right = formatInOffset(b.end, offsetInfo.minutes, false, false, '')
        return `${left} – ${right} • ${humanBytes(b.bytesIn + b.bytesOut)}`
    }

    // Start–end label for an arbitrary window (used by the live drag readout).
    const windowLabel = (startMs: number, endMs: number) => {
        const left = formatInOffset(startMs, offsetInfo.minutes, true, true, offsetInfo.suffix)
        const right = formatInOffset(endMs, offsetInfo.minutes, false, false, '')
        return `${left} – ${right}`
    }

    // Map a pointer x-coordinate to a tile index, clamped to the grid so a drag
    // past either edge of the bar pins to the first/last tile.
    const indexFromClientX = (clientX: number): number => {
        const el = rowRef.current
        if (!el) return 0
        const rect = el.getBoundingClientRect()
        const frac = rect.width > 0 ? (clientX - rect.left) / rect.width : 0
        let idx = Math.floor(frac * tileCount)
        if (idx < 0) idx = 0
        if (idx >= tileCount) idx = tileCount - 1
        return idx
    }

    const onRowPointerDown = (e: React.PointerEvent) => {
        if (e.button !== 0) return
        e.preventDefault()
        const idx = indexFromClientX(e.clientX)
        const prior = selection
        const wasSingle =
            !!prior &&
            prior.anchor === prior.focus &&
            Math.min(prior.anchor, prior.focus) === idx
        press.current = { idx, moved: false, wasSingle }
        dragging.current = true
        setIsDragging(true)
        // Focus the bar so Escape (handled on the container) clears the selection
        // even after a mouse drag, where no tile button receives focus.
        rowRef.current?.focus()
        try {
            rowRef.current?.setPointerCapture(e.pointerId)
        } catch {
            /* capture unsupported — drag still works within the bar */
        }
        setSelection({ anchor: idx, focus: idx })
    }

    const onRowPointerMove = (e: React.PointerEvent) => {
        if (!dragging.current) return
        const idx = indexFromClientX(e.clientX)
        if (idx !== press.current.idx) press.current.moved = true
        setSelection((prev) => (prev ? { anchor: prev.anchor, focus: idx } : prev))
    }

    const onRowPointerUp = (e: React.PointerEvent) => {
        if (!dragging.current) return
        dragging.current = false
        setIsDragging(false)
        try {
            rowRef.current?.releasePointerCapture(e.pointerId)
        } catch {
            /* nothing captured */
        }
        // a no-move click on the already-selected single tile toggles it off
        if (!press.current.moved && press.current.wasSingle) setSelection(null)
    }

    // Keyboard path: Enter/Space on a focused tile selects (or toggles off) a single tile.
    const selectSingle = (idx: number) => {
        setSelection((prev) =>
            prev && prev.anchor === prev.focus && prev.anchor === idx
                ? null
                : { anchor: idx, focus: idx }
        )
    }

    const fmtEdge = (ms: number) => formatTimestamp(new Date(ms).toISOString())

    // Determine if the executed drill-down is "service" style (no IPs, has dport or proto)
    const isServiceDrill = React.useMemo(() => {
        const s = new Set(drill?.attrs ?? [])
        return s.size > 0 && !s.has('sip') && !s.has('dip') && (s.has('dport') || s.has('proto'))
    }, [drill?.attrs])

    // Selected bucket ring color depends on directionality
    const selectedRingClass = (b: Bucket) => {
        const uni = b.bytesIn === 0 || b.bytesOut === 0
        return uni ? 'ring-2 ring-red-300' : 'ring-2 ring-primary-300'
    }

    return (
        <tr>
            <td colSpan={colSpan} className="p-0">
                <div className="mx-3 my-2 rounded-xl bg-surface-200 ring-1 ring-line shadow-md border-l-2 border-primary-400/70 px-4 py-3">
                    {error ? (
                        <div className="mb-2 rounded-md border border-red-500/40 bg-red-500/10 px-2 py-1 text-data text-red-200">
                            {renderError(error)}
                        </div>
                    ) : null}

                    {/* Single-row timeline */}
                    {!loading && rows.length > 0 && (
                        <div className="mb-2">
                            <div
                                ref={rowRef}
                                tabIndex={-1}
                                className="relative flex gap-px touch-none select-none focus:outline-none"
                                onPointerDown={onRowPointerDown}
                                onPointerMove={onRowPointerMove}
                                onPointerUp={onRowPointerUp}
                                onPointerCancel={onRowPointerUp}
                                onKeyDown={(ev) => {
                                    if (ev.key === 'Escape' && range !== null) {
                                        ev.preventDefault()
                                        setSelection(null)
                                    }
                                }}
                            >
                                {buckets.map((b, i) => {
                                    const hasData = b.rowIndices.length > 0
                                    // Single-tile selections keep the directional ring; a multi-tile
                                    // range is drawn as one spanning band overlay (below) instead.
                                    const isSingle =
                                        range !== null && range.lo === range.hi && i === range.lo
                                    return (
                                        <button
                                            key={i}
                                            type="button"
                                            className="h-2 flex-1 min-w-0"
                                            onKeyDown={(ev) => {
                                                if (ev.key === 'Enter' || ev.key === ' ') {
                                                    ev.preventDefault()
                                                    selectSingle(i)
                                                }
                                            }}
                                            aria-label={tileTitle(b)}
                                            title={tileTitle(b)}
                                        >
                                            <div
                                                className={`h-full w-full rounded-[2px] ring-1 ${tileColorClass(b)} ${isSingle ? selectedRingClass(b) : ''} ${hasData ? 'cursor-pointer hover:ring-primary-300/80' : 'opacity-60'}`}
                                            />
                                        </button>
                                    )
                                })}
                                {/* Spanning band for a multi-tile range. Positioned with the same
                                    uniform tileCount fraction model as indexFromClientX, so the band
                                    stays aligned with where pointer hits land. */}
                                {range !== null && range.lo < range.hi && (
                                    <div
                                        className="pointer-events-none absolute inset-y-0 rounded-[2px] bg-primary-400/25 ring-2 ring-primary-300"
                                        style={{
                                            left: `${(range.lo / tileCount) * 100}%`,
                                            width: `${((range.hi - range.lo + 1) / tileCount) * 100}%`,
                                        }}
                                    />
                                )}
                            </div>
                            <div className="mt-0.5 flex justify-between text-data-xs text-gray-400 select-none">
                                <span>{fmtEdge(firstMs)}</span>
                                <span>{fmtEdge(lastMs)}</span>
                            </div>
                        </div>
                    )}

                    {loading && (
                        <div className="py-4 text-center text-data text-gray-400">Loading…</div>
                    )}

                    {/* Live drag readout: cheap window + total, defers the full card to commit */}
                    {!loading && range !== null && isDragging && (() => {
                        const lo = buckets[range.lo]
                        const hi = buckets[range.hi]
                        if (!lo || !hi) return null
                        let total = 0
                        for (let bi = range.lo; bi <= range.hi; bi++) {
                            const b = buckets[bi]
                            if (b) total += b.bytesIn + b.bytesOut
                        }
                        return (
                            <div className="mt-2 flex items-center gap-2 text-data text-gray-300 select-none">
                                <span className="tabular-nums">{windowLabel(lo.start, hi.end)}</span>
                                <span className="text-gray-500">·</span>
                                <span className="tabular-nums text-accent font-medium">{humanBytes(total)}</span>
                            </div>
                        )
                    })()}

                    {/* Selected interval: full-width totals, then auto-run drill-down results */}
                    {!loading && range !== null && !isDragging && (() => {
                        const lo = buckets[range.lo]
                        const hi = buckets[range.hi]
                        if (!lo || !hi) return null
                        // Union of rows across the selected tile span (literal endpoints).
                        const idxs: number[] = []
                        for (let bi = range.lo; bi <= range.hi; bi++) {
                            const b = buckets[bi]
                            if (b) for (const ri of b.rowIndices) idxs.push(ri)
                        }
                        if (idxs.length === 0) return null
                        let inB = 0, outB = 0, inP = 0, outP = 0
                        for (const ri of idxs) {
                            const r = rows[ri]
                            inB += r.bytes_in || 0
                            outB += r.bytes_out || 0
                            inP += r.packets_in || 0
                            outP += r.packets_out || 0
                        }
                        const totalB = inB + outB
                        const totalP = inP + outP
                        const firstRow = rows[idxs[0]]
                        const lastRow = rows[idxs[idxs.length - 1]]
                        const { label } = buildIntervalLabel(firstRow.interval_end || '', durMs, undefined)
                        const heading = idxs.length === 1
                            ? label
                            : (() => {
                                const { label: lastLabel } = buildIntervalLabel(
                                    lastRow.interval_end || '', durMs, undefined
                                )
                                return `${label} … ${lastLabel}`
                            })()
                        const uni = (inB + inP === 0) || (outB + outP === 0)
                        const backgroundClass = uni
                            ? 'bg-red-400/15 ring-1 ring-red-400/20'
                            : 'bg-surface-200/60 border-line'
                        const showDrillResults =
                            drill?.phase === 'done' &&
                            drill.bucket.startMs === lo.start &&
                            drill.bucket.endMs === hi.end

                        return (
                            <div className="mt-2">
                                <div className="relative z-10">
                                    {/* Interval totals (full width) */}
                                    <DetailsCard
                                        heading={heading}
                                        totalBytes={totalB}
                                        totalPackets={totalP}
                                        inBytes={inB}
                                        inPackets={inP}
                                        outBytes={outB}
                                        outPackets={outP}
                                        parentTotalBytes={rowTotals.bytes}
                                        parentTotalPackets={rowTotals.packets}
                                        backgroundClass={backgroundClass}
                                        className="rounded-xl px-4 py-3"
                                    />
                                </div>

                                {/* Drill-down results */}
                                {drillError != null && (
                                    <div className="mt-2 rounded-md border border-red-500/40 bg-red-500/10 px-2 py-1 text-data text-red-200">
                                        {renderError(drillError)}
                                    </div>
                                )}
                                {drillLoading && (
                                    <div className="mt-2 py-3 text-center text-data text-gray-400">Loading drill-down…</div>
                                )}
                                {showDrillResults && drill && (
                                    <div className="mt-2">
                                        {drill.rows.length === 0 ? (
                                            <div className="text-data text-gray-400 text-center py-2">No results</div>
                                        ) : isServiceDrill ? (
                                            <DrillServiceResults flows={drill.rows} parentTotalBytes={totalB} parentTotalPackets={totalP} />
                                        ) : (
                                            <DrillTableResults flows={drill.rows} attrs={drill.attrs} visualInOutBars={visualInOutBars} showDirectionValues={showDirectionValues} />
                                        )}
                                    </div>
                                )}
                            </div>
                        )
                    })()}
                </div>
            </td>
        </tr>
    )
}

// Render drill-down results as service cards (proto/port)
function DrillServiceResults({ flows, parentTotalBytes, parentTotalPackets }: { flows: FlowRecord[]; parentTotalBytes: number; parentTotalPackets: number }) {
    const groups = groupByService(flows)
    return (
        <div>
            <div className="mb-1 text-data-xs uppercase tracking-wide text-gray-400">Services</div>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4">
                {groups.map((g, i) => {
                    const uni = (g.inB + g.inP === 0) || (g.outB + g.outP === 0)
                    const bg = uni ? 'bg-red-400/15 ring-1 ring-red-400/20' : undefined
                    return (
                        <ServiceDetailsCard
                            key={i}
                            proto={g.proto}
                            dport={g.dport}
                            inBytes={g.inB}
                            inPackets={g.inP}
                            outBytes={g.outB}
                            outPackets={g.outP}
                            parentTotalBytes={parentTotalBytes}
                            parentTotalPackets={parentTotalPackets}
                            backgroundClass={bg}
                            className="rounded-lg px-3 py-2"
                        />
                    )
                })}
            </div>
        </div>
    )
}

// Render drill-down results as a compact table
function DrillTableResults({
    flows,
    attrs,
    visualInOutBars = false,
    showDirectionValues = false,
}: {
    flows: FlowRecord[]
    attrs: string[]
    visualInOutBars?: boolean
    showDirectionValues?: boolean
}) {
    const showSip = attrs.includes('sip')
    const showDip = attrs.includes('dip')
    const showDport = attrs.includes('dport')
    const showProto = attrs.includes('proto')
    // shared scales across the drill-down rows (this is an independent result
    // set, so it carries its own max — see Q2/Q9)
    const bytesScaleMax = React.useMemo(
        () => inOutScaleMax(flows, 'bytes_in', 'bytes_out'),
        [flows],
    )
    const packetsScaleMax = React.useMemo(
        () => inOutScaleMax(flows, 'packets_in', 'packets_out'),
        [flows],
    )

    return (
        <div className="max-h-60 overflow-auto scroll-thin">
            <table className="min-w-full border-collapse text-left text-sm">
                <thead className="table-header text-xs uppercase tracking-wide text-gray-400 sticky top-0 bg-surface-200">
                    <tr>
                        {showSip && <th className="px-2 py-1 font-medium align-bottom">sip</th>}
                        {showDip && <th className="px-2 py-1 font-medium align-bottom">dip</th>}
                        {showDport && <th className="px-2 py-1 font-medium text-right align-bottom">dport</th>}
                        {showProto && <th className="px-2 py-1 font-medium align-bottom">proto</th>}
                        {visualInOutBars ? (
                            <>
                                <th className="px-2 py-1 font-medium">
                                    <div className="mx-auto flex w-40 flex-col leading-tight text-gray-400">
                                        <span className="text-center text-gray-300">bytes</span>
                                        <span className="flex w-full items-center">
                                            <span className="w-1/2 pr-1 text-right">in ◄</span>
                                            <span className="w-1/2 pl-1 text-left">► out</span>
                                        </span>
                                    </div>
                                </th>
                                <th className="px-2 py-1 font-medium text-right align-bottom">total</th>
                                <th className="px-2 py-1 font-medium">
                                    <div className="mx-auto flex w-40 flex-col leading-tight text-gray-400">
                                        <span className="text-center text-gray-300">packets</span>
                                        <span className="flex w-full items-center">
                                            <span className="w-1/2 pr-1 text-right">in ◄</span>
                                            <span className="w-1/2 pl-1 text-left">► out</span>
                                        </span>
                                    </div>
                                </th>
                                <th className="px-2 py-1 font-medium text-right align-bottom">total</th>
                            </>
                        ) : (
                            <>
                                <th className="px-2 py-1 font-medium text-right">bytes in</th>
                                <th className="px-2 py-1 font-medium text-right">bytes out</th>
                                <th className="px-2 py-1 font-medium text-right">bytes total</th>
                                <th className="px-2 py-1 font-medium text-right">packets total</th>
                            </>
                        )}
                    </tr>
                </thead>
                <tbody className="divide-y divide-line-soft">
                    {flows.map((r, i) => {
                        const uni = !r.bidirectional
                        return (
                            <tr key={i} className={uni && !visualInOutBars ? 'bg-red-400/10' : ''}>
                                {showSip && <td className="px-2 py-1 text-data">{r.sip}</td>}
                                {showDip && <td className="px-2 py-1 text-data">{r.dip}</td>}
                                {showDport && <td className="px-2 py-1 tabular-nums text-right text-data">{r.dport ?? ''}</td>}
                                {showProto && <td className="px-2 py-1 text-data">{renderProto(r.proto)}</td>}
                                {visualInOutBars ? (
                                    <>
                                        <td className="px-2 py-1">
                                            <InOutBar
                                                inValue={r.bytes_in}
                                                outValue={r.bytes_out}
                                                scaleMax={bytesScaleMax}
                                                unidirectional={uni}
                                                showValues={showDirectionValues}
                                                format={humanBytes}
                                            />
                                        </td>
                                        <td className="px-2 py-1 tabular-nums text-right text-data text-accent font-medium">{humanBytes(r.bytes_total)}</td>
                                        <td className="px-2 py-1">
                                            <InOutBar
                                                inValue={r.packets_in}
                                                outValue={r.packets_out}
                                                scaleMax={packetsScaleMax}
                                                unidirectional={uni}
                                                showValues={showDirectionValues}
                                                format={humanPackets}
                                            />
                                        </td>
                                        <td className="px-2 py-1 tabular-nums text-right text-data text-accent">{humanPackets(r.packets_total)}</td>
                                    </>
                                ) : (
                                    <>
                                        <td className="px-2 py-1 tabular-nums text-right text-data text-gray-300">{humanBytes(r.bytes_in)}</td>
                                        <td className="px-2 py-1 tabular-nums text-right text-data text-gray-300">{humanBytes(r.bytes_out)}</td>
                                        <td className="px-2 py-1 tabular-nums text-right text-data text-accent font-medium">{humanBytes(r.bytes_total)}</td>
                                        <td className="px-2 py-1 tabular-nums text-right text-data text-accent">{humanPackets(r.packets_total)}</td>
                                    </>
                                )}
                            </tr>
                        )
                    })}
                </tbody>
            </table>
        </div>
    )
}
