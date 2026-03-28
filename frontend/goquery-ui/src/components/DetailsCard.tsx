import React from 'react'
import { humanBytes, humanPackets, bytesOrEmpty, pktsOrEmpty } from '../utils/format'
import { ProgressBar } from './ProgressBar'
import { usePanelTotals } from './DetailsPanel'

export interface DetailsCardProps {
  heading: string
  totalBytes: number
  totalPackets: number
  inBytes: number
  inPackets: number
  outBytes: number
  outPackets: number
  // explicit parent totals (used when DetailsCard is outside a DetailsPanel)
  parentTotalBytes?: number
  parentTotalPackets?: number
  // background styling classes (e.g., bg-surface-200/60 border-white/10 or red variant)
  backgroundClass?: string
  // optional container className overrides (e.g., rounded, padding, highlight rings)
  className?: string
}

export const DetailsCard: React.FC<DetailsCardProps> = ({
  heading,
  totalBytes,
  totalPackets,
  inBytes,
  inPackets,
  outBytes,
  outPackets,
  parentTotalBytes,
  parentTotalPackets,
  backgroundClass,
  className,
}) => {
  // prefer context (from DetailsPanel), fall back to explicit props
  const panelTotals = usePanelTotals()
  const ptBytes = panelTotals?.totalBytes ?? parentTotalBytes
  const ptPackets = panelTotals?.totalPackets ?? parentTotalPackets
  const bg = backgroundClass || 'bg-surface-200/60 border-white/10'
  const container = `rounded-lg border p-3 ${bg} ${className || ''}`.trim()

  const bytesPct = ptBytes && ptBytes > 0 ? (totalBytes / ptBytes) * 100 : undefined
  const pktsPct = ptPackets && ptPackets > 0 ? (totalPackets / ptPackets) * 100 : undefined

  return (
    <div className={container}>
      <div className="mb-2 flex items-center gap-2">
        <div className="min-w-0 shrink text-data font-medium text-white truncate">{heading}</div>
        <div className="ml-auto flex shrink-0 items-end gap-2">
          {bytesPct !== undefined && (
            <div className="flex w-[90px] flex-col items-end">
              <span className="text-data font-medium text-primary-300 whitespace-nowrap">
                {humanBytes(totalBytes)}
              </span>
              <ProgressBar percent={bytesPct} />
            </div>
          )}
          {pktsPct !== undefined && (
            <div className="flex w-[90px] flex-col items-end">
              <span className="text-data font-medium text-primary-300 whitespace-nowrap">
                {humanPackets(totalPackets)}
              </span>
              <ProgressBar percent={pktsPct} />
            </div>
          )}
        </div>
        {bytesPct === undefined && (
          <div className="text-right text-data font-medium text-primary-300 whitespace-nowrap">
            {humanBytes(totalBytes)} / {humanPackets(totalPackets)}
          </div>
        )}
      </div>
      <div className="grid grid-cols-2 gap-2 text-data">
        <div>
          <div className="mb-0.5 text-data-xs uppercase tracking-wide text-gray-400">In</div>
          <div className="text-gray-100">{bytesOrEmpty(inBytes)}</div>
          <div className="text-primary-300">{pktsOrEmpty(inPackets)}</div>
        </div>
        <div>
          <div className="mb-0.5 text-data-xs uppercase tracking-wide text-gray-400">Out</div>
          <div className="text-gray-100">{bytesOrEmpty(outBytes)}</div>
          <div className="text-primary-300">{pktsOrEmpty(outPackets)}</div>
        </div>
      </div>
    </div>
  )
}

export default DetailsCard
