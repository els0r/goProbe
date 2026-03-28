import { DetailsPanel } from '../components/DetailsPanel'
import { FlowRecord, SummarySchema } from '../api/domain'
import { DisplaySummary } from '../components/DisplaySummary'
import { IfaceDetails } from '../components/IfaceDetails'
import { sumTotals, groupByIface } from '../utils/aggregation'
import { renderError } from '../utils/renderError'
import { usePanelEscape } from '../hooks/usePanelEscape'

export interface HostDetailsPanelProps {
  host: string
  loading: boolean
  error?: unknown
  rows: FlowRecord[]
  summary?: SummarySchema
  onClose: () => void
}

export function HostDetailsPanel({
  host,
  rows,
  loading,
  error,
  summary,
  onClose,
}: HostDetailsPanelProps) {
  usePanelEscape(onClose)
  const totals = sumTotals(rows)
  const byIface = groupByIface(rows)

  return (
    <div className="absolute right-2 top-2 z-20 w-[360px] max-h-[66vh] overflow-hidden">
      <DetailsPanel
        title={host}
        totalBytes={totals.inB + totals.outB}
        totalPackets={totals.inP + totals.outP}
        onClose={onClose}
      >
        {error ? (
          <div className="mb-2 rounded-md border border-red-500/40 bg-red-500/10 px-2 py-1 text-data text-red-200">
            {renderError(error)}
          </div>
        ) : null}
        <div className="mb-1 text-data-sm font-semibold uppercase tracking-wide text-gray-300">
          Interfaces
        </div>
        {summary && (
          <DisplaySummary displayed={summary.hits.displayed} total={summary.hits.total} />
        )}
        <IfaceDetails groups={byIface} loading={loading} />
      </DetailsPanel>
    </div>
  )
}
