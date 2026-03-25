import React from 'react'
import { FlowRecord, SummarySchema } from '../api/domain'
import { DetailsCard } from '../components/DetailsCard'
import { InOutSummary } from '../components/InOutSummary'
import { humanBytes, humanPackets } from '../utils/format'
import { renderError } from '../utils/renderError'
import {
    formatShortDuration,
    extractOffset,
    formatInOffset,
    buildIntervalLabel,
} from '../utils/temporal'

export interface TemporalInlineMeta {
    host: string
    iface: string
    sip: string
    dip: string
    dport?: number | null
    proto?: number | null
}

export interface TemporalInlineDetailProps {
    meta: TemporalInlineMeta
    rows: FlowRecord[]
    loading: boolean
    error?: unknown
    summary?: SummarySchema
    colSpan: number
}

export function TemporalInlineDetail({
    meta,
    rows,
    loading,
    error,
    summary,
    colSpan,
}: TemporalInlineDetailProps) {
    const listRef = React.useRef<HTMLDivElement | null>(null)
    const rowRefs = React.useRef<Array<HTMLDivElement | null>>([])
    const [highlightIndex, setHighlightIndex] = React.useState<number | null>(null)

    const totalIn = rows.reduce((s, r) => s + (r.bytes_in || 0), 0)
    const totalOut = rows.reduce((s, r) => s + (r.bytes_out || 0), 0)
    const totalPktsIn = rows.reduce((s, r) => s + (r.packets_in || 0), 0)
    const totalPktsOut = rows.reduce((s, r) => s + (r.packets_out || 0), 0)

    const times = rows.map((r) => r.interval_end || '').filter(Boolean)
    const uniqueTimes = Array.from(new Set(times))

    const resolutionNs = (summary as any)?.timings?.resolution
    const durMs: number = Number.isFinite(resolutionNs)
        ? Math.max(1, Math.round(Number(resolutionNs) / 1_000_000))
        : 5 * 60 * 1000

    const tfFirst = (summary as any)?.time_first as string | undefined
    const tfLast = (summary as any)?.time_last as string | undefined
    const lastMs = tfLast
        ? Date.parse(tfLast)
        : times.length
            ? Math.max(...times.map((t) => Date.parse(t)))
            : Date.now()
    const firstMs = tfFirst
        ? Date.parse(tfFirst)
        : times.length
            ? Math.min(...times.map((t) => Date.parse(t)))
            : lastMs - 12 * 60 * 60 * 1000
    const queryDurMs = Math.max(1, lastMs - firstMs)
    const niceDurationsMs = [
        5 * 60 * 1000, 10 * 60 * 1000, 20 * 60 * 1000, 30 * 60 * 1000,
        60 * 60 * 1000, 2 * 60 * 60 * 1000, 4 * 60 * 60 * 1000, 6 * 60 * 60 * 1000,
        12 * 60 * 60 * 1000, 24 * 60 * 60 * 1000, 48 * 60 * 60 * 1000,
    ] as const
    const requiredPerTile = Math.ceil(queryDurMs / (6 * 24))
    const tileMs =
        niceDurationsMs.find((d) => d >= requiredPerTile) || niceDurationsMs[niceDurationsMs.length - 1]
    const gridTileCount = 6 * 24
    const gridSpanMs = tileMs * gridTileCount
    const gridEndMs = lastMs
    const gridStartMs = gridEndMs - gridSpanMs
    const offsetInfo = extractOffset(tfLast || times[0] || new Date().toISOString())

    const buckets: Array<{ start: number; end: number; bytes: number; firstRowIndex?: number }> =
        Array.from({ length: gridTileCount }, (_, i) => {
            const start = gridStartMs + i * tileMs
            return { start, end: start + tileMs, bytes: 0 }
        })
    rows.forEach((r, i) => {
        const e = r.interval_end ? Date.parse(r.interval_end) : NaN
        if (!Number.isFinite(e)) return
        if (e <= gridStartMs || e > gridEndMs) return
        const pos = (e - gridStartMs) / tileMs
        let idx = Math.ceil(pos) - 1
        if (idx < 0) idx = 0
        if (idx >= gridTileCount) idx = gridTileCount - 1
        const bt = (r.bytes_in || 0) + (r.bytes_out || 0)
        buckets[idx].bytes += bt
        if (buckets[idx].firstRowIndex === undefined) buckets[idx].firstRowIndex = i
    })
    const maxBucket = buckets.reduce((m, b) => Math.max(m, b.bytes), 0)

    const classForShare = (share: number) => {
        if (!Number.isFinite(share) || share <= 0) return 'bg-surface-100 ring-white/10'
        if (share < 0.2) return 'bg-primary-500/20 ring-primary-500/30'
        if (share < 0.4) return 'bg-primary-500/35 ring-primary-500/40'
        if (share < 0.6) return 'bg-primary-500/55 ring-primary-500/50'
        if (share < 0.8) return 'bg-primary-500/70 ring-primary-500/60'
        return 'bg-primary-500/90 ring-primary-500/70'
    }
    const tileClass = (b: { bytes: number }) => classForShare(maxBucket ? b.bytes / maxBucket : 0)
    const tileTitle = (b: { start: number; end: number; bytes: number }) => {
        const left = formatInOffset(b.start, offsetInfo.minutes, true, true, offsetInfo.suffix)
        const right = formatInOffset(b.end, offsetInfo.minutes, false, false, '')
        return `${left} – ${right} • ${humanBytes(b.bytes)}`
    }
    const onTileClick = (b: { firstRowIndex?: number }, e?: React.MouseEvent) => {
        e?.preventDefault()
        e?.stopPropagation()
        const idx = b.firstRowIndex
        if (idx === undefined || idx === null) return
        const target = rowRefs.current[idx]
        const container = listRef.current
        if (!target || !container) return
        const contRect = container.getBoundingClientRect()
        const targRect = target.getBoundingClientRect()
        const delta = targRect.top - contRect.top
        const top = container.scrollTop + delta - 6
        container.scrollTo({ top, behavior: 'smooth' })
        setHighlightIndex(idx)
        window.setTimeout(() => setHighlightIndex(null), 1500)
    }

    return (
        <tr>
            <td colSpan={colSpan} className="p-0">
                <div className="border-t border-white/10 bg-surface-100/60 px-4 py-3">
                    {/* Header */}
                    <div className="mb-2 flex items-baseline justify-between gap-3">
                        <div className="text-data font-semibold text-white" title={meta.host}>
                            {meta.host} — {meta.iface}
                        </div>
                        <div className="text-right text-data font-medium text-primary-300 whitespace-nowrap">
                            {humanBytes(totalIn + totalOut)} / {humanPackets(totalPktsIn + totalPktsOut)}
                        </div>
                    </div>

                    {error ? (
                        <div className="mb-2 rounded-md border border-red-500/40 bg-red-500/10 px-2 py-1 text-data text-red-200">
                            {renderError(error)}
                        </div>
                    ) : null}

                    <InOutSummary
                        inBytes={totalIn}
                        inPackets={totalPktsIn}
                        outBytes={totalOut}
                        outPackets={totalPktsOut}
                    />

                    {/* Occurrence grid (6×24) */}
                    {!loading && rows.length > 0 && (
                        <>
                            <div className="mb-2 relative">
                                <div className="absolute left-0 top-0 bottom-0 w-10 select-none">
                                    <div className="grid h-full grid-rows-6 content-between pr-1 text-right text-data-xs text-gray-400">
                                        {Array.from({ length: 6 }).map((_, i) => {
                                            let label = ''
                                            if (i === 2) label = formatShortDuration(3 * tileMs)
                                            if (i === 5) label = formatShortDuration(6 * tileMs)
                                            return (
                                                <div key={i} className="flex items-center justify-end">
                                                    {label}
                                                </div>
                                            )
                                        })}
                                    </div>
                                </div>
                                <div className="pl-10">
                                    <div
                                        className="grid gap-[3px] sm:gap-1"
                                        style={{ gridTemplateColumns: 'repeat(24, minmax(0, 1fr))' }}
                                    >
                                        {buckets.map((b, i) => {
                                            const col = Math.floor(i / 6) + 1
                                            const row = (i % 6) + 1
                                            return (
                                                <button
                                                    key={i}
                                                    type="button"
                                                    className="relative w-full pt-[100%]"
                                                    onClick={(ev) => onTileClick(b, ev)}
                                                    aria-label={tileTitle(b)}
                                                    title={tileTitle(b)}
                                                    style={{ gridColumn: String(col), gridRow: String(row) }}
                                                >
                                                    <div
                                                        className={`absolute inset-0 rounded-[3px] ring-1 ${tileClass(b)} ${b.firstRowIndex !== undefined ? 'cursor-pointer hover:ring-primary-300/80' : 'opacity-60'}`}
                                                    />
                                                </button>
                                            )
                                        })}
                                    </div>
                                </div>
                            </div>
                            {/* X-axis labels */}
                            {(() => {
                                const labels: Array<{ col: number; text: string }> = []
                                const spanHours = gridSpanMs / 3600000
                                const rowSpanHours = (tileMs * 6) / 3600000
                                const fmtAxis = (h: number) =>
                                    h > 24 ? `${Math.round(h / 24)}d` : `${Math.round(h)}h`

                                const addLabel = (hours: number) => {
                                    const col = Math.min(23, Math.max(0, Math.round((hours / spanHours) * 24) - 1))
                                    if (!labels.some((l) => Math.abs(l.col - col) < 2))
                                        labels.push({ col, text: fmtAxis(hours) })
                                }
                                addLabel(rowSpanHours)
                                for (let h = 24; h <= spanHours; h += 24) addLabel(h)
                                const rightText = fmtAxis(spanHours)
                                if (!labels.some((l) => l.col === 23)) labels.push({ col: 23, text: rightText })

                                if (!labels.length) return null
                                const cells = Array.from(
                                    { length: 24 },
                                    (_, i) => labels.find((l) => l.col === i)?.text || ''
                                )
                                return (
                                    <div className="mb-2 select-none text-data-xs text-gray-400 pl-10">
                                        <div
                                            className="grid"
                                            style={{ gridTemplateColumns: 'repeat(24, minmax(0, 1fr))' }}
                                        >
                                            {cells.map((txt, i) => (
                                                <div key={i} className="text-center">
                                                    {txt}
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                )
                            })()}
                        </>
                    )}

                    {/* Interval list */}
                    <div className="mb-1 text-data text-gray-300">
                        {loading ? (
                            'Loading…'
                        ) : (
                            <>
                                Found in <span className="font-semibold text-white">{uniqueTimes.length}</span> time
                                intervals
                            </>
                        )}
                    </div>
                    <div ref={listRef} className="scroll-thin max-h-[40vh] overflow-auto pr-1">
                        {!loading &&
                            rows.map((r, i) => {
                                const totalB = (r.bytes_in || 0) + (r.bytes_out || 0)
                                const totalP = (r.packets_in || 0) + (r.packets_out || 0)
                                const endIso = r.interval_end || ''
                                const prevEndIso = i > 0 ? rows[i - 1]?.interval_end || '' : undefined
                                const { label, isNewDate } = buildIntervalLabel(endIso, durMs, prevEndIso)
                                const uni =
                                    (r.bytes_in || 0) + (r.packets_in || 0) === 0 ||
                                    (r.bytes_out || 0) + (r.packets_out || 0) === 0
                                const baseClass = isNewDate
                                    ? 'bg-surface-200/60 border-white/10'
                                    : 'bg-surface-100/60 border-white/10'
                                const backgroundClass = uni
                                    ? 'bg-red-400/15 ring-1 ring-red-400/20 hover:bg-red-400/25'
                                    : baseClass
                                return (
                                    <div
                                        key={i}
                                        ref={(el) => { rowRefs.current[i] = el }}
                                        className={`mb-2 ${highlightIndex === i ? 'ring-2 ring-primary-500 rounded-xl' : ''}`}
                                        data-unidirectional={uni || undefined}
                                    >
                                        <DetailsCard
                                            heading={label}
                                            totalBytes={totalB}
                                            totalPackets={totalP}
                                            inBytes={r.bytes_in || 0}
                                            inPackets={r.packets_in || 0}
                                            outBytes={r.bytes_out || 0}
                                            outPackets={r.packets_out || 0}
                                            backgroundClass={backgroundClass}
                                            className="rounded-xl px-4 py-3"
                                        />
                                    </div>
                                )
                            })}
                    </div>
                </div>
            </td>
        </tr>
    )
}
