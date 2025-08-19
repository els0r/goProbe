import React from 'react'
import { humanBytes, humanPackets } from '../utils/format'

export interface DetailsPanelProps {
    title: React.ReactNode
    totalBytes?: number
    totalPackets?: number
    className?: string
    children?: React.ReactNode
    onClose?: () => void
}


export function DetailsPanel({ title, totalBytes, totalPackets, className, children, onClose }: DetailsPanelProps) {
    return (
        <div className={(className ? className + ' ' : '') + 'rounded-xl border border-white/10 bg-surface-100/80 shadow-xl backdrop-blur-md'}>
            <div className="mb-2 flex items-baseline justify-between gap-3 px-3 pt-3">
                <div className="text-[12px] font-semibold text-white">{title}</div>
                {(totalBytes !== undefined || totalPackets !== undefined) && (
                    <div className="text-right text-[12px] font-medium text-primary-300 whitespace-nowrap">
                        {humanBytes(totalBytes ?? 0)} / {humanPackets(totalPackets ?? 0)}
                    </div>
                )}
            </div>
            <div className="px-3 pb-3">
                {children}
                {onClose && (
                    <div className="mt-3 text-right">
                        <button onClick={onClose} className="rounded-md bg-surface-200 px-2 py-1 text-[12px] ring-1 ring-white/10 hover:bg-surface-300">Close</button>
                    </div>
                )}
            </div>
        </div>
    )
}

export default DetailsPanel
