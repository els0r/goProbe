import React, { createContext, useContext } from 'react'
import { humanBytes, humanPackets } from '../utils/format'

/** Context that DetailsPanel provides so child DetailsCards can render share bars. */
interface PanelTotals {
  totalBytes: number
  totalPackets: number
}
const PanelTotalsCtx = createContext<PanelTotals | null>(null)
export const usePanelTotals = () => useContext(PanelTotalsCtx)

export interface DetailsPanelProps {
  title: React.ReactNode
  totalBytes?: number
  totalPackets?: number
  className?: string
  children?: React.ReactNode
  onClose?: () => void
}

export function DetailsPanel({
  title,
  totalBytes,
  totalPackets,
  className,
  children,
  onClose,
}: DetailsPanelProps) {
  const ctx: PanelTotals | null =
    totalBytes !== undefined && totalPackets !== undefined
      ? { totalBytes, totalPackets }
      : null

  return (
    <PanelTotalsCtx.Provider value={ctx}>
      <div
        className={
          (className ? className + ' ' : '') +
          'rounded-xl border border-white/10 bg-surface-100/80 shadow-xl backdrop-blur-md'
        }
      >
        <div className="mb-2 flex items-baseline justify-between gap-3 px-3 pt-3">
          <div className="text-data font-semibold text-white">{title}</div>
          {ctx && (
            <div className="text-right text-data font-medium text-primary-300 whitespace-nowrap">
              {humanBytes(ctx.totalBytes)} / {humanPackets(ctx.totalPackets)}
            </div>
          )}
        </div>
        <div className="px-3 pb-3">
          {children}
          {onClose && (
            <div className="mt-3 text-right">
              <button
                onClick={onClose}
                className="rounded-md bg-surface-200 px-2 py-1 text-data ring-1 ring-white/10 hover:bg-surface-300"
              >
                Close
              </button>
            </div>
          )}
        </div>
      </div>
    </PanelTotalsCtx.Provider>
  )
}

export default DetailsPanel
