import React from 'react'
import { bytesOrEmpty, pktsOrEmpty } from '../utils/format'

export interface InOutSummaryProps {
  inBytes: number
  inPackets: number
  outBytes: number
  outPackets: number
  className?: string
}

export const InOutSummary: React.FC<InOutSummaryProps> = ({
  inBytes,
  inPackets,
  outBytes,
  outPackets,
  className,
}) => {
  return (
    <div className={(className ? className + ' ' : '') + 'mb-3 grid grid-cols-2 gap-2 text-data'}>
      <div className="rounded-md bg-surface-200/40 p-2 ring-1 ring-line-soft">
        <div className="mb-0.5 text-data-xs uppercase tracking-wide text-gray-400">In</div>
        <div className="text-gray-100">{bytesOrEmpty(inBytes)}</div>
        <div className="text-accent">{pktsOrEmpty(inPackets)}</div>
      </div>
      <div className="rounded-md bg-surface-200/40 p-2 ring-1 ring-line-soft">
        <div className="mb-0.5 text-data-xs uppercase tracking-wide text-gray-400">Out</div>
        <div className="text-gray-100">{bytesOrEmpty(outBytes)}</div>
        <div className="text-accent">{pktsOrEmpty(outPackets)}</div>
      </div>
    </div>
  )
}

export default InOutSummary
