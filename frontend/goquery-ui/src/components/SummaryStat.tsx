import React from 'react'

export interface SummaryStatProps {
  label: string
  value: React.ReactNode
  multiline?: boolean
}

export function SummaryStat({ label, value, multiline }: SummaryStatProps) {
  const isSimple = typeof value === 'string' || typeof value === 'number'
  const valueClass = multiline
    ? 'text-[13px] font-medium text-gray-100 leading-tight break-words'
    : 'truncate text-[13px] font-medium text-gray-100'
  return (
    <div className="flex flex-col rounded-md bg-surface-200/40 px-2 py-2 ring-1 ring-white/5">
      <div className="mb-0.5 text-data-xs uppercase tracking-wide text-gray-400">{label}</div>
      <div className={valueClass} title={isSimple ? String(value) : undefined}>
        {value}
      </div>
    </div>
  )
}
