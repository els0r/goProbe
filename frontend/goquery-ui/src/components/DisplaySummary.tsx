import React from 'react'

export interface DisplaySummaryProps {
  displayed?: number
  total?: number
  className?: string
}

export const DisplaySummary: React.FC<DisplaySummaryProps> = ({ displayed, total, className }) => {
  if (displayed === undefined && total === undefined) return null
  const cls = `mb-2 text-[11px] text-gray-400 ${className || ''}`.trim()
  if (total !== undefined) {
    return (
      <div className={cls}>
        Displayed <span className="font-semibold text-white">{displayed ?? 0}</span> out of{' '}
        <span className="font-semibold text-white">{total}</span> results
      </div>
    )
  }
  return (
    <div className={cls}>
      Displayed <span className="font-semibold text-white">{displayed ?? 0}</span> results
    </div>
  )
}

export default DisplaySummary
