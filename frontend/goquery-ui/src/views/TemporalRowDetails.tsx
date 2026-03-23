import React from 'react'
import { FlowRecord, SummarySchema } from '../api/domain'
import { DetailsCard } from '../components/DetailsCard'
import { humanBytes } from '../utils/format'
import { renderError } from '../utils/renderError'
import { extractOffset, formatInOffset, buildIntervalLabel } from '../utils/temporal'
import { formatTimestamp } from '../utils/timeFormat'

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
}

/**
 * Maximum number of tiles in the single-row timeline.
 * Keeps the tiles readable. The actual count may be smaller
 * when the query window is short relative to resolution.
 */
const MAX_TILES = 144

export function TemporalRowDetails({
    rows,
    loading,
    error,
    summary,
    colSpan,
    queryFirst,
    queryLast,
}: TemporalRowDetailsProps) {
    const [selectedBucket, setSelectedBucket] = React.useState<number | null>(null)

    const times = rows.map((r) => r.interval_end || '').filter(Boolean)

    const resolutionNs = (summary as any)?.timings?.resolution
    const durMs: number = Number.isFinite(resolutionNs)
        ? Math.max(1, Math.round(Number(resolutionNs) / 1_000_000))
        : 5 * 60 * 1000

    // Timeline bounds from query params, falling back to summary, then data
    const firstMs = queryFirst
        ? Date.parse(queryFirst)
        : (summary as any)?.time_first
            ? Date.parse((summary as any).time_first)
            : times.length
                ? Math.min(...times.map((t) => Date.parse(t)))
                : Date.now() - 12 * 60 * 60 * 1000
    const lastMs = queryLast
        ? Date.parse(queryLast)
        : (summary as any)?.time_last
            ? Date.parse((summary as any).time_last)
            : times.length
                ? Math.max(...times.map((t) => Date.parse(t)))
                : Date.now()

    const spanMs = Math.max(1, lastMs - firstMs)
    // Each tile covers one resolution interval; cap at MAX_TILES
    const rawTileCount = Math.max(1, Math.ceil(spanMs / durMs))
    const tileCount = Math.min(rawTileCount, MAX_TILES)
    const tileMs = spanMs / tileCount

    const offsetInfo = extractOffset(
        queryLast || (summary as any)?.time_last || times[0] || new Date().toISOString()
    )

    // Build buckets spanning [firstMs, lastMs)
    const buckets: Array<{ start: number; end: number; bytes: number; rowIndices: number[] }> =
        Array.from({ length: tileCount }, (_, i) => {
            const start = firstMs + i * tileMs
            return { start, end: start + tileMs, bytes: 0, rowIndices: [] }
        })

    rows.forEach((r, i) => {
        const e = r.interval_end ? Date.parse(r.interval_end) : NaN
        if (!Number.isFinite(e)) return
        if (e <= firstMs || e > lastMs) return
        let idx = Math.floor((e - firstMs) / tileMs)
        if (idx < 0) idx = 0
        if (idx >= tileCount) idx = tileCount - 1
        buckets[idx].bytes += (r.bytes_in || 0) + (r.bytes_out || 0)
        buckets[idx].rowIndices.push(i)
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

    const onTileClick = (bucketIdx: number, e?: React.MouseEvent) => {
        e?.preventDefault()
        e?.stopPropagation()
        // Toggle: clicking the same bucket again hides it
        setSelectedBucket((prev) => (prev === bucketIdx ? null : bucketIdx))
    }

    // X-axis: first and last timestamps (same format as summary TIME RANGE)
    const fmtEdge = (ms: number) => formatTimestamp(new Date(ms).toISOString())

    return (
        <tr>
            <td colSpan={colSpan} className="p-0">
                <div className="border-t border-white/10 bg-surface-100/60 px-4 py-3">
                    {error ? (
                        <div className="mb-2 rounded-md border border-red-500/40 bg-red-500/10 px-2 py-1 text-data text-red-200">
                            {renderError(error)}
                        </div>
                    ) : null}

                    {/* Single-row timeline */}
                    {!loading && rows.length > 0 && (
                        <div className="mb-2">
                            <div className="flex gap-px">
                                {buckets.map((b, i) => (
                                    <button
                                        key={i}
                                        type="button"
                                        className="h-3 flex-1 min-w-0"
                                        onClick={(ev) => onTileClick(i, ev)}
                                        aria-label={tileTitle(b)}
                                        title={tileTitle(b)}
                                    >
                                        <div
                                            className={`h-full w-full rounded-[2px] ring-1 ${tileClass(b)} ${selectedBucket === i ? 'ring-2 ring-primary-300' : ''} ${b.rowIndices.length > 0 ? 'cursor-pointer hover:ring-primary-300/80' : 'opacity-60'}`}
                                        />
                                    </button>
                                ))}
                            </div>
                            {/* Axis labels */}
                            <div className="mt-0.5 flex justify-between text-data-xs text-gray-400 select-none">
                                <span>{fmtEdge(firstMs)}</span>
                                <span>{fmtEdge(lastMs)}</span>
                            </div>
                        </div>
                    )}

                    {/* Selected interval detail */}
                    {loading && (
                        <div className="py-4 text-center text-data text-gray-400">Loading…</div>
                    )}
                    {!loading && selectedBucket !== null && (() => {
                        const bucket = buckets[selectedBucket]
                        if (!bucket || bucket.rowIndices.length === 0) return null
                        // Aggregate all rows in this bucket into a single card
                        let inB = 0, outB = 0, inP = 0, outP = 0
                        for (const ri of bucket.rowIndices) {
                            const r = rows[ri]
                            inB += r.bytes_in || 0
                            outB += r.bytes_out || 0
                            inP += r.packets_in || 0
                            outP += r.packets_out || 0
                        }
                        const totalB = inB + outB
                        const totalP = inP + outP
                        const firstRow = rows[bucket.rowIndices[0]]
                        const lastRow = rows[bucket.rowIndices[bucket.rowIndices.length - 1]]
                        const endIso = firstRow.interval_end || ''
                        const { label } = buildIntervalLabel(endIso, durMs, undefined)
                        // If multiple rows in bucket, show range
                        const heading = bucket.rowIndices.length === 1
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
                            : 'bg-surface-200/60 border-white/10'
                        return (
                            <div className="mt-2" data-unidirectional={uni || undefined}>
                                <DetailsCard
                                    heading={heading}
                                    totalBytes={totalB}
                                    totalPackets={totalP}
                                    inBytes={inB}
                                    inPackets={inP}
                                    outBytes={outB}
                                    outPackets={outP}
                                    backgroundClass={backgroundClass}
                                    className="rounded-xl px-4 py-3"
                                />
                            </div>
                        )
                    })()}
                </div>
            </td>
        </tr>
    )
}
