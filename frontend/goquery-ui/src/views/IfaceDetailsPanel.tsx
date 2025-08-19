import { useEffect } from 'react'
import { FlowRecord, SummarySchema } from '../api/domain'
import { ServiceDetails } from '../components/ServiceDetails'
import { DetailsPanel } from '../components/DetailsPanel'
import { DisplaySummary } from '../components/DisplaySummary'
import { InOutSummary } from '../components/InOutSummary'

export interface IfaceDetailsPanelProps {
  host: string
  iface: string
  loading: boolean
  error?: unknown
  rows: FlowRecord[]
  summary?: SummarySchema
  onClose: () => void
}

export function IfaceDetailsPanel({ host, iface, rows, loading, error, summary, onClose }: IfaceDetailsPanelProps) {
  // close on Escape
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const totals = rows.reduce((acc, r) => {
    acc.inB += r.bytes_in; acc.outB += r.bytes_out
    acc.inP += r.packets_in; acc.outP += r.packets_out
    return acc
  }, { inB: 0, outB: 0, inP: 0, outP: 0 })

  const groups = (() => {
    const m = new Map<string, { proto: number | null; dport: number | null; inB: number; outB: number; inP: number; outP: number }>()
    for (const r of rows) {
      const key = `${r.proto ?? 'na'}|${r.dport ?? 'na'}`
      const g = m.get(key) || { proto: r.proto ?? null, dport: r.dport ?? null, inB: 0, outB: 0, inP: 0, outP: 0 }
      g.inB += r.bytes_in; g.outB += r.bytes_out
      g.inP += r.packets_in; g.outP += r.packets_out
      m.set(key, g)
    }
    return Array.from(m.values()).sort((a, b) => (b.inB + b.outB) - (a.inB + a.outB))
  })()

  const renderError = (e: unknown): string => {
    if (!e) return ''
    if (typeof e === 'string') return e
    try { return String((e as any)?.message || JSON.stringify(e)) } catch { return 'Error' }
  }

  return (
    <div className="absolute right-2 top-2 z-20 w-[360px] max-h-[66vh] overflow-hidden">
      <DetailsPanel title={<span>{host} — {iface}</span>} totalBytes={totals.inB + totals.outB} totalPackets={totals.inP + totals.outP} onClose={onClose}>
        {error ? (
          <div className="mb-2 rounded-md border border-red-500/40 bg-red-500/10 px-2 py-1 text-[12px] text-red-200">{renderError(error)}</div>
        ) : null}
        <InOutSummary inBytes={totals.inB} inPackets={totals.inP} outBytes={totals.outB} outPackets={totals.outP} />
        <div className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-gray-300">Services</div>
        {summary && (
          <DisplaySummary displayed={summary.hits.displayed} total={summary.hits.total} />
        )}
        <ServiceDetails groups={groups} loading={loading} />
      </DetailsPanel>
    </div>
  )
}
