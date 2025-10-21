import React, { useEffect } from 'react'
import { FlowRecord, SummarySchema } from '../api/domain'
import { DetailsCard } from '../components/DetailsCard'
import { renderProto } from '../utils/proto'
import { humanBytes } from '../utils/format'
import { DetailsPanel } from '../components/DetailsPanel'
import { InOutSummary } from '../components/InOutSummary'

export interface TemporalMeta {
  host: string
  iface: string
  sip: string
  dip: string
  dport?: number | null
  proto?: number | null
}

export interface TemporalDetailsPanelProps {
  meta: TemporalMeta
  attrsShown: string[]
  rows: FlowRecord[]
  loading: boolean
  error?: unknown
  summary?: SummarySchema
  onClose: () => void
}

function formatShortDuration(ms: number): string {
  const minutes = Math.round(ms / 60000)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.round(minutes / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.round(hours / 24)
  return `${days}d`
}

// derive interval labels using the fact that interval_end is the end of the bucket.
// we preserve the timezone from the original ISO string for display.
function extractOffset(iso: string): { suffix: string; minutes: number } {
  const m = (iso || '').match(/(Z|[+-]\d{2}:?\d{2})$/)
  if (!m) return { suffix: '', minutes: 0 }
  const token = m[1]
  if (token === 'Z') return { suffix: 'Z', minutes: 0 }
  const sign = token[0] === '-' ? -1 : 1
  const hh = Number(token.slice(1, 3))
  const mm = Number(token.slice(-2))
  const suffix = `${sign === 1 ? '+' : '-'}${String(hh).padStart(2, '0')}:${String(mm).padStart(2, '0')}`
  return { suffix, minutes: sign * (hh * 60 + mm) }
}

function formatInOffset(
  ms: number,
  offsetMin: number,
  includeDate: boolean,
  includeSuffix: boolean,
  suffix: string
): string {
  const dt = new Date(ms + offsetMin * 60 * 1000) // shift so UTC getters reflect wall time for the offset
  const Y = dt.getUTCFullYear()
  const M = String(dt.getUTCMonth() + 1).padStart(2, '0')
  const D = String(dt.getUTCDate()).padStart(2, '0')
  const h = String(dt.getUTCHours()).padStart(2, '0')
  const m = String(dt.getUTCMinutes()).padStart(2, '0')
  const s = String(dt.getUTCSeconds()).padStart(2, '0')
  const datePart = `${Y}-${M}-${D} `
  const timePart = `${h}:${m}:${s}`
  return (includeDate ? datePart : '') + timePart + (includeSuffix ? suffix : '')
}

function dateKeyInOffset(ms: number, offsetMin: number): string {
  const dt = new Date(ms + offsetMin * 60 * 1000)
  const Y = dt.getUTCFullYear()
  const M = String(dt.getUTCMonth() + 1).padStart(2, '0')
  const D = String(dt.getUTCDate()).padStart(2, '0')
  return `${Y}-${M}-${D}`
}

function buildIntervalLabel(
  endIso: string,
  durMs: number,
  prevEndIso?: string
): { label: string; isNewDate: boolean } {
  if (!endIso) return { label: '', isNewDate: false }
  const endMs = Date.parse(endIso)
  if (!Number.isFinite(endMs)) return { label: endIso, isNewDate: false }
  const { suffix, minutes } = extractOffset(endIso)
  const startMs = endMs - Math.max(1, durMs)
  const endDay = dateKeyInOffset(endMs, minutes)
  const prevDay =
    prevEndIso && Number.isFinite(Date.parse(prevEndIso))
      ? dateKeyInOffset(Date.parse(prevEndIso as string), minutes)
      : null
  const isNewDate = !prevDay || prevDay !== endDay
  const left = formatInOffset(startMs, minutes, isNewDate, isNewDate, suffix)
  const right = formatInOffset(endMs, minutes, false, false, '')
  return { label: `${left} – ${right}`, isNewDate }
}

export function TemporalDetailsPanel({
  meta,
  attrsShown,
  rows,
  loading,
  error,
  summary,
  onClose,
}: TemporalDetailsPanelProps) {
  // close on Escape
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])
  const listRef = React.useRef<HTMLDivElement | null>(null)
  const rowRefs = React.useRef<Array<HTMLDivElement | null>>([])
  const [highlightIndex, setHighlightIndex] = React.useState<number | null>(null)
  const totalIn = rows.reduce((s, r) => s + (r.bytes_in || 0), 0)
  const totalOut = rows.reduce((s, r) => s + (r.bytes_out || 0), 0)
  const totalPktsIn = rows.reduce((s, r) => s + (r.packets_in || 0), 0)
  const totalPktsOut = rows.reduce((s, r) => s + (r.packets_out || 0), 0)

  const times = rows.map((r) => r.interval_end || '').filter(Boolean)
  const uniqueTimes = Array.from(new Set(times))

  // interval duration (ns in summary); fallback to 5 minutes if unknown
  const resolutionNs = (summary as any)?.timings?.resolution
  const durMs: number = Number.isFinite(resolutionNs)
    ? Math.max(1, Math.round(Number(resolutionNs) / 1_000_000))
    : 5 * 60 * 1000

  // occurrence grid (6 x 24)
  const tfFirst = (summary as any)?.time_first as string | undefined
  const tfLast = (summary as any)?.time_last as string | undefined
  // derive query duration and choose the smallest "nice" tile so 144 tiles cover the span
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
    5 * 60 * 1000, // 5m
    10 * 60 * 1000, // 10m
    20 * 60 * 1000, // 20m
    30 * 60 * 1000, // 30m
    60 * 60 * 1000, // 1h
    2 * 60 * 60 * 1000, // 2h
    4 * 60 * 60 * 1000, // 4h
    6 * 60 * 60 * 1000, // 6h
    12 * 60 * 60 * 1000, // 12h
    24 * 60 * 60 * 1000, // 24h
    48 * 60 * 60 * 1000, // 48h
  ] as const
  const requiredPerTile = Math.ceil(queryDurMs / (6 * 24))
  let tileMs =
    niceDurationsMs.find((d) => d >= requiredPerTile) || niceDurationsMs[niceDurationsMs.length - 1]
  const gridTileCount = 6 * 24
  const gridSpanMs = tileMs * gridTileCount
  const gridEndMs = lastMs
  const gridStartMs = gridEndMs - gridSpanMs
  const offsetInfo = extractOffset(tfLast || times[0] || new Date().toISOString())
  // initialize buckets
  const buckets: Array<{
    start: number
    end: number
    bytes: number
    firstRowIndex?: number
  }> = Array.from({ length: gridTileCount }, (_, i) => {
    const start = gridStartMs + i * tileMs
    return { start, end: start + tileMs, bytes: 0 }
  })
  // aggregate rows into buckets based on interval_end (belongs to tile where it ends)
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
  // buckets currently in chronological order (oldest to newest)
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

  const renderError = (e: unknown): string => {
    if (!e) return ''
    if (typeof e === 'string') return e
    try {
      return String((e as any)?.message || JSON.stringify(e))
    } catch {
      return 'Error'
    }
  }

  return (
    <div className="absolute right-2 top-2 z-20 w-[680px] max-h-[70vh]">
      <DetailsPanel
        title={
          <span title={meta.host}>
            {meta.host} — {meta.iface}
          </span>
        }
        totalBytes={totalIn + totalOut}
        totalPackets={totalPktsIn + totalPktsOut}
        className="bg-surface-100/90"
        onClose={onClose}
      >
        {/* Attributes in a single row (only those visible in the table) */}
        <div className="mb-3 rounded-md bg-surface-200/40 px-3 py-2 text-[12px] ring-1 ring-white/10">
          <div className="flex flex-wrap items-baseline gap-x-6 gap-y-1">
            {attrsShown.includes('sip') && (
              <div>
                <span className="mr-1 text-gray-400">SIP</span>
                <span className="font-mono break-all">{meta.sip}</span>
              </div>
            )}
            {attrsShown.includes('dip') && (
              <div>
                <span className="mr-1 text-gray-400">DIP</span>
                <span className="font-mono break-all">{meta.dip}</span>
              </div>
            )}
            {attrsShown.includes('dport') && meta.dport !== undefined && meta.dport !== null && (
              <div>
                <span className="mr-1 text-gray-400">DPORT</span>
                <span className="font-mono">{meta.dport}</span>
              </div>
            )}
            {attrsShown.includes('proto') && meta.proto !== undefined && meta.proto !== null && (
              <div>
                <span className="mr-1 text-gray-400">PROTO</span>
                <span className="font-mono">{renderProto(meta.proto)}</span>
              </div>
            )}
          </div>
        </div>
        {error ? (
          <div className="mb-2 rounded-md border border-red-500/40 bg-red-500/10 px-2 py-1 text-[12px] text-red-200">
            {renderError(error)}
          </div>
        ) : null}
        {/* totals in/out summary */}
        <InOutSummary
          inBytes={totalIn}
          inPackets={totalPktsIn}
          outBytes={totalOut}
          outPackets={totalPktsOut}
        />
        {/* Occurrence View (6 x 24 grid) */}
        <div className="mb-2 relative">
          {/* Left Y-axis labels */}
          <div className="absolute left-0 top-0 bottom-0 w-10 select-none">
            <div className="grid h-full grid-rows-6 content-between pr-1 text-right text-[10px] text-gray-400">
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
          {/* Grid with left padding to make space for Y-axis */}
          <div className="pl-10">
            <div
              className="grid gap-[3px] sm:gap-1"
              style={{ gridTemplateColumns: 'repeat(24, minmax(0, 1fr))' }}
            >
              {buckets.map((b, i) => {
                const col = Math.floor(i / 6) + 1 // 1..24
                const row = (i % 6) + 1 // 1..6
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
        {/* X-axis time markers */}
        {(() => {
          const labels: Array<{ col: number; text: string }> = []
          const spanHours = gridSpanMs / 3600000
          const rowSpanHours = (tileMs * 6) / 3600000
          const fmtAxis = (h: number) => (h > 24 ? `${Math.round(h / 24)}d` : `${Math.round(h)}h`)

          const addLabel = (hours: number) => {
            const col = Math.min(23, Math.max(0, Math.round((hours / spanHours) * 24) - 1))
            if (!labels.some((l) => Math.abs(l.col - col) < 2))
              labels.push({ col, text: fmtAxis(hours) })
          }

          // left: row height (e.g., 6h)
          addLabel(rowSpanHours)
          // day ticks every 24h up to span
          for (let h = 24; h <= spanHours; h += 24) addLabel(h)

          // rightmost: full span
          const rightText = fmtAxis(spanHours)
          if (!labels.some((l) => l.col === 23)) labels.push({ col: 23, text: rightText })

          if (!labels.length) return null
          const cells = Array.from(
            { length: 24 },
            (_, i) => labels.find((l) => l.col === i)?.text || ''
          )
          return (
            <div className="mb-2 select-none text-[10px] text-gray-400 pl-10">
              <div className="grid" style={{ gridTemplateColumns: 'repeat(24, minmax(0, 1fr))' }}>
                {cells.map((txt, i) => (
                  <div key={i} className="text-center">
                    {txt}
                  </div>
                ))}
              </div>
            </div>
          )
        })()}
        <div className="mb-1 text-[12px] text-gray-300">
          Found in <span className="font-semibold text-white">{uniqueTimes.length}</span> time
          intervals
        </div>
        <div ref={listRef} className="scroll-thin max-h-[50vh] overflow-auto pr-1">
          {loading && <div className="py-8 text-center text-[12px] text-gray-400">Loading…</div>}
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
                  ref={(el) => (rowRefs.current[i] = el)}
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
      </DetailsPanel>
    </div>
  )
}
