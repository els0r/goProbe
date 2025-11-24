import React from 'react'
import { humanBytes, humanPackets, bytesOrEmpty, pktsOrEmpty } from '../utils/format'

export interface DetailsCardProps {
  heading: string
  totalBytes: number
  totalPackets: number
  inBytes: number
  inPackets: number
  outBytes: number
  outPackets: number
  // background styling classes (e.g., bg-surface-200/60 border-white/10 or red variant)
  backgroundClass?: string
  // optional container className overrides (e.g., rounded, padding, highlight rings)
  className?: string
}

// wrappers imported from utils/format

export const DetailsCard: React.FC<DetailsCardProps> = ({
  heading,
  totalBytes,
  totalPackets,
  inBytes,
  inPackets,
  outBytes,
  outPackets,
  backgroundClass,
  className,
}) => {
  const bg = backgroundClass || 'bg-surface-200/60 border-white/10'
  const container = `rounded-lg border p-3 ${bg} ${className || ''}`.trim()

  return (
    <div className={container}>
      <div className="mb-2 flex items-baseline justify-between gap-3">
        <div className="text-[12px] font-medium text-white">{heading}</div>
        <div className="text-right text-[12px] font-medium text-primary-300 whitespace-nowrap">
          {humanBytes(totalBytes)} / {humanPackets(totalPackets)}
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2 text-[12px]">
        <div>
          <div className="mb-0.5 text-[10px] uppercase tracking-wide text-gray-400">In</div>
          <div className="text-gray-100">{bytesOrEmpty(inBytes)}</div>
          <div className="text-primary-300">{pktsOrEmpty(inPackets)}</div>
        </div>
        <div>
          <div className="mb-0.5 text-[10px] uppercase tracking-wide text-gray-400">Out</div>
          <div className="text-gray-100">{bytesOrEmpty(outBytes)}</div>
          <div className="text-primary-300">{pktsOrEmpty(outPackets)}</div>
        </div>
      </div>
    </div>
  )
}

export default DetailsCard
