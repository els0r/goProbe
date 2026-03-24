import React from 'react'
import { FlowRecord, QueryParamsUI, SummarySchema } from '../api/domain'
import { getGlobalQueryClient } from '../api/client'
import { DetailsCard } from '../components/DetailsCard'
import { ServiceDetailsCard } from '../components/ServiceDetailsCard'
import { AttributesSelect, AttributeOption, buildAttributeQuery } from '../components/AttributesSelect'
import { humanBytes, humanPackets } from '../utils/format'
import { renderError } from '../utils/renderError'
import { renderProto } from '../utils/proto'
import { extractOffset, formatInOffset, buildIntervalLabel } from '../utils/temporal'
import { formatTimestamp } from '../utils/timeFormat'
import { groupByService } from '../utils/aggregation'

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
    /** Row identity for condition building */
    meta?: {
        host: string
        host_id: string
        iface: string
        sip: string
        dip: string
        dport?: number | null
        proto?: number | null
    }
    /** Attributes used in the original query */
    attrsShown?: string[]
    /** Condition from the original query (e.g. "proto eq icmp") */
    originalCondition?: string
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
    originalCondition,
}: TemporalRowDetailsProps) {
    const [selectedBucket, setSelectedBucket] = React.useState<number | null>(null)

    // Drill-down state
    const complementAttrs = React.useMemo(() => {
        const shown = new Set(attrsShown || [])
        return ATTR_OPTIONS.map((o) => o.value).filter((v) => !shown.has(v))
    }, [attrsShown])

    const [drillAttrs, setDrillAttrs] = React.useState<string[]>(complementAttrs)
    const [drillAllSelected, setDrillAllSelected] = React.useState(false)
    const [drillLoading, setDrillLoading] = React.useState(false)
    const [drillError, setDrillError] = React.useState<unknown>(null)
    const [drillFlows, setDrillFlows] = React.useState<FlowRecord[] | null>(null)
    const [drillSummary, setDrillSummary] = React.useState<SummarySchema | undefined>()
    // Track which bucket the drill results belong to
    const [drillBucketIdx, setDrillBucketIdx] = React.useState<number | null>(null)

    // Reset drill-down attrs when complement changes
    React.useEffect(() => {
        setDrillAttrs(complementAttrs)
        setDrillAllSelected(false)
    }, [complementAttrs])

    const times = rows.map((r) => r.interval_end || '').filter(Boolean)

    const resolutionNs = (summary as any)?.timings?.resolution
    const durMs: number = Number.isFinite(resolutionNs)
        ? Math.max(1, Math.round(Number(resolutionNs) / 1_000_000))
        : 5 * 60 * 1000

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

    // Color: blue for bidirectional, red for uni-directional
    const tileColorClass = (b: Bucket) => {
        const total = b.bytesIn + b.bytesOut
        const share = maxBucket > 0 ? total / maxBucket : 0
        if (!Number.isFinite(share) || share <= 0) return 'bg-surface-100 ring-white/10'
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

    const onTileClick = (bucketIdx: number, e?: React.MouseEvent) => {
        e?.preventDefault()
        e?.stopPropagation()
        setSelectedBucket((prev) => {
            const next = prev === bucketIdx ? null : bucketIdx
            if (next !== drillBucketIdx) {
                // Clear drill results when switching tiles
                setDrillFlows(null)
                setDrillError(null)
                setDrillBucketIdx(null)
            }
            return next
        })
    }

    const fmtEdge = (ms: number) => formatTimestamp(new Date(ms).toISOString())

    // Drill-down query execution
    const runDrillDown = async (bucket: Bucket) => {
        if (!meta) return
        const effectiveAttrs = drillAllSelected
            ? ATTR_OPTIONS.map((o) => o.value)
            : drillAttrs
        if (effectiveAttrs.length === 0) return

        const condParts: string[] = []
        if (meta.sip) condParts.push(`sip=${meta.sip}`)
        if (meta.dip) condParts.push(`dip=${meta.dip}`)
        if (meta.dport !== null && meta.dport !== undefined) condParts.push(`dport=${meta.dport}`)
        if (meta.proto !== null && meta.proto !== undefined) condParts.push(`proto=${meta.proto}`)
        // Include the original query condition (e.g. "proto eq icmp")
        if (originalCondition) condParts.push(originalCondition)

        const drillParams: QueryParamsUI = {
            first: new Date(bucket.start).toISOString(),
            last: new Date(bucket.end).toISOString(),
            ifaces: meta.iface || '',
            query: buildAttributeQuery(effectiveAttrs, drillAllSelected),
            query_hosts: meta.host_id || undefined,
            hosts_resolver: 'string',
            condition: condParts.join(' and ') || undefined,
            limit: 1000,
            sort_by: 'bytes',
            sort_ascending: false,
        }

        setDrillLoading(true)
        setDrillError(null)
        setDrillFlows(null)
        setDrillBucketIdx(selectedBucket)

        try {
            const data = await getGlobalQueryClient().runQueryUI(drillParams)
            setDrillFlows(data.flows)
            setDrillSummary(data.summary)
        } catch (e) {
            setDrillError(e)
        } finally {
            setDrillLoading(false)
        }
    }

    // Determine if drill-down attrs are "service" style (only dport+proto or just dport or just proto)
    const isServiceDrill = React.useMemo(() => {
        const effective = drillAllSelected ? ATTR_OPTIONS.map((o) => o.value) : drillAttrs
        const s = new Set(effective)
        return s.size > 0 && !s.has('sip') && !s.has('dip') && (s.has('dport') || s.has('proto'))
    }, [drillAttrs, drillAllSelected])

    // Selected bucket ring color depends on directionality
    const selectedRingClass = (b: Bucket) => {
        const uni = b.bytesIn === 0 || b.bytesOut === 0
        return uni ? 'ring-2 ring-red-300' : 'ring-2 ring-primary-300'
    }

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
                                {buckets.map((b, i) => {
                                    const hasData = b.rowIndices.length > 0
                                    const isSelected = selectedBucket === i
                                    return (
                                        <button
                                            key={i}
                                            type="button"
                                            className="h-3 flex-1 min-w-0"
                                            onClick={(ev) => onTileClick(i, ev)}
                                            aria-label={tileTitle(b)}
                                            title={tileTitle(b)}
                                        >
                                            <div
                                                className={`h-full w-full rounded-[2px] ring-1 ${tileColorClass(b)} ${isSelected ? selectedRingClass(b) : ''} ${hasData ? 'cursor-pointer hover:ring-primary-300/80' : 'opacity-60'}`}
                                            />
                                        </button>
                                    )
                                })}
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

                    {/* Selected interval: detail card (left) + drill-down (right) */}
                    {!loading && selectedBucket !== null && (() => {
                        const bucket = buckets[selectedBucket]
                        if (!bucket || bucket.rowIndices.length === 0) return null
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
                        const effectiveAttrs = drillAllSelected
                            ? ATTR_OPTIONS.map((o) => o.value)
                            : drillAttrs
                        const hasDrillAttrs = effectiveAttrs.length > 0
                        const showDrillResults = drillFlows !== null && drillBucketIdx === selectedBucket

                        return (
                            <div className="mt-2">
                                <div className="relative z-10 grid grid-cols-2 gap-3">
                                    {/* Left: interval totals */}
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
                                    {/* Right: drill-down attributes selector */}
                                    <div className="rounded-xl border border-white/10 bg-surface-200/60 px-4 py-3">
                                        <div className="mb-2 text-data-xs uppercase tracking-wide text-gray-400">
                                            Drill down
                                        </div>
                                        <div className="flex items-center gap-2">
                                            <div className="flex-1">
                                                <AttributesSelect
                                                    options={ATTR_OPTIONS}
                                                    value={drillAttrs}
                                                    allSelected={drillAllSelected}
                                                    dropUp
                                                    onChange={(next) => {
                                                        setDrillAttrs(next.values)
                                                        setDrillAllSelected(next.all)
                                                    }}
                                                />
                                            </div>
                                            <button
                                                type="button"
                                                disabled={!hasDrillAttrs || drillLoading}
                                                onClick={() => runDrillDown(bucket)}
                                                className="rounded-md bg-primary-500 px-3 py-1 text-data font-medium text-white hover:bg-primary-400 disabled:opacity-40 disabled:cursor-not-allowed whitespace-nowrap"
                                            >
                                                {drillLoading ? 'Running…' : 'Run'}
                                            </button>
                                        </div>
                                        {drillError != null && (
                                            <div className="mt-2 rounded-md border border-red-500/40 bg-red-500/10 px-2 py-1 text-data text-red-200">
                                                {renderError(drillError)}
                                            </div>
                                        )}
                                    </div>
                                </div>

                                {/* Drill-down results */}
                                {drillLoading && (
                                    <div className="mt-2 py-3 text-center text-data text-gray-400">Loading drill-down…</div>
                                )}
                                {showDrillResults && drillFlows && (
                                    <div className="mt-2">
                                        {drillFlows.length === 0 ? (
                                            <div className="text-data text-gray-400 text-center py-2">No results</div>
                                        ) : isServiceDrill ? (
                                            <DrillServiceResults flows={drillFlows} />
                                        ) : (
                                            <DrillTableResults flows={drillFlows} attrs={effectiveAttrs} />
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
function DrillServiceResults({ flows }: { flows: FlowRecord[] }) {
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
function DrillTableResults({ flows, attrs }: { flows: FlowRecord[]; attrs: string[] }) {
    const showSip = attrs.includes('sip')
    const showDip = attrs.includes('dip')
    const showDport = attrs.includes('dport')
    const showProto = attrs.includes('proto')

    return (
        <div className="max-h-60 overflow-auto scroll-thin">
            <table className="min-w-full border-collapse text-left text-sm">
                <thead className="table-header text-xs uppercase tracking-wide text-gray-400 sticky top-0 bg-surface-100">
                    <tr>
                        {showSip && <th className="px-2 py-1 font-medium">sip</th>}
                        {showDip && <th className="px-2 py-1 font-medium">dip</th>}
                        {showDport && <th className="px-2 py-1 font-medium text-right">dport</th>}
                        {showProto && <th className="px-2 py-1 font-medium">proto</th>}
                        <th className="px-2 py-1 font-medium text-right">bytes in</th>
                        <th className="px-2 py-1 font-medium text-right">bytes out</th>
                        <th className="px-2 py-1 font-medium text-right">bytes total</th>
                        <th className="px-2 py-1 font-medium text-right">packets total</th>
                    </tr>
                </thead>
                <tbody className="divide-y divide-white/5">
                    {flows.map((r, i) => {
                        const uni = !r.bidirectional
                        return (
                            <tr key={i} className={uni ? 'bg-red-400/10' : ''}>
                                {showSip && <td className="px-2 py-1 text-data">{r.sip}</td>}
                                {showDip && <td className="px-2 py-1 text-data">{r.dip}</td>}
                                {showDport && <td className="px-2 py-1 tabular-nums text-right text-data">{r.dport ?? ''}</td>}
                                {showProto && <td className="px-2 py-1 text-data">{renderProto(r.proto)}</td>}
                                <td className="px-2 py-1 tabular-nums text-right text-data text-gray-300">{humanBytes(r.bytes_in)}</td>
                                <td className="px-2 py-1 tabular-nums text-right text-data text-gray-300">{humanBytes(r.bytes_out)}</td>
                                <td className="px-2 py-1 tabular-nums text-right text-data text-primary-300 font-medium">{humanBytes(r.bytes_in + r.bytes_out)}</td>
                                <td className="px-2 py-1 tabular-nums text-right text-data text-primary-300">{humanPackets(r.packets_in + r.packets_out)}</td>
                            </tr>
                        )
                    })}
                </tbody>
            </table>
        </div>
    )
}
