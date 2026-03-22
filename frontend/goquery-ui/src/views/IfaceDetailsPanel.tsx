import { FlowRecord, SummarySchema } from '../api/domain'
import { ServiceDetails } from '../components/ServiceDetails'
import { DetailsPanel } from '../components/DetailsPanel'
import { DisplaySummary } from '../components/DisplaySummary'
import { InOutSummary } from '../components/InOutSummary'
import { sumTotals, groupByService } from '../utils/aggregation'
import { renderError } from '../utils/renderError'
import { usePanelEscape } from '../hooks/usePanelEscape'

export interface IfaceDetailsPanelProps {
  host: string
  iface: string
  loading: boolean
  error?: unknown
  rows: FlowRecord[]
  summary?: SummarySchema
  onClose: () => void
}

export function IfaceDetailsPanel({
  host,
  iface,
  rows,
  loading,
  error,
  summary,
  onClose,
}: IfaceDetailsPanelProps) {
  usePanelEscape(onClose)
  const totals = sumTotals(rows)
  const groups = groupByService(rows)

  return (
    <div className="absolute right-2 top-2 z-20 w-[360px] max-h-[66vh] overflow-hidden">
      <DetailsPanel
        title={
          <span>
            {host} — {iface}
          </span>
        }
        totalBytes={totals.inB + totals.outB}
        totalPackets={totals.inP + totals.outP}
        onClose={onClose}
      >
        {error ? (
          <div className="mb-2 rounded-md border border-red-500/40 bg-red-500/10 px-2 py-1 text-data text-red-200">
            {renderError(error)}
          </div>
        ) : null}
        <InOutSummary
          inBytes={totals.inB}
          inPackets={totals.inP}
          outBytes={totals.outB}
          outPackets={totals.outP}
        />
        <div className="mb-1 text-data-sm font-semibold uppercase tracking-wide text-gray-300">
          Services
        </div>
        {summary && (
          <DisplaySummary displayed={summary.hits.displayed} total={summary.hits.total} />
        )}
        <ServiceDetails groups={groups} loading={loading} />
      </DetailsPanel>
    </div>
  )
}
