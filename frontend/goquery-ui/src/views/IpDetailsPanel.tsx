import { FlowRecord, SummarySchema } from '../api/domain'
import { ServiceDetails } from '../components/ServiceDetails'
import { DetailsPanel } from '../components/DetailsPanel'
import { DisplaySummary } from '../components/DisplaySummary'
import { InOutSummary } from '../components/InOutSummary'
import { sumTotals, groupByService } from '../utils/aggregation'
import { renderError } from '../utils/renderError'
import { usePanelEscape } from '../hooks/usePanelEscape'

export interface IpDetailsPanelProps {
  ip: string
  loading: boolean
  error?: unknown
  rows: FlowRecord[]
  onClose: () => void
  summary?: SummarySchema
}

export function IpDetailsPanel({
  ip,
  rows,
  loading,
  error,
  onClose,
  summary,
}: IpDetailsPanelProps) {
  usePanelEscape(onClose)
  const totals = sumTotals(rows)
  const groups = groupByService(rows)

  return (
    <div className="absolute right-2 top-2 z-20 w-[360px] max-h-[66vh] overflow-hidden">
      <DetailsPanel
        title={ip}
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
