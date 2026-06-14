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
          'flex max-h-[66vh] flex-col rounded-xl border border-line bg-surface-100 shadow-xl'
        }
      >
        <div className="flex shrink-0 items-baseline justify-between gap-3 px-3 pb-2 pt-3">
          <div className="text-data font-semibold text-gray-100">{title}</div>
          {ctx && (
            <div className="text-right text-data font-medium text-accent whitespace-nowrap">
              {humanBytes(ctx.totalBytes)} / {humanPackets(ctx.totalPackets)}
            </div>
          )}
        </div>
        {/* scrolling body; header above and Close below stay pinned so the
            action is always reachable regardless of list length */}
        <div className="min-h-0 flex-1 overflow-y-auto scroll-thin px-3 pb-3">
          {children}
        </div>
        {onClose && (
          <div className="shrink-0 border-t border-line px-3 py-2 text-right">
            <button
              onClick={onClose}
              className="rounded-md bg-surface-200 px-2 py-1 text-data ring-1 ring-line hover:bg-surface-300"
            >
              Close
            </button>
          </div>
        )}
      </div>
    </PanelTotalsCtx.Provider>
  )
}

export default DetailsPanel
