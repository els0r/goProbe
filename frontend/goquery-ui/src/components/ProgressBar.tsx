import React from 'react'

export interface ProgressBarProps {
  // linear percentage in [0,100]
  percent: number
  // optional title attribute override; defaults to `${percent.toFixed(1)} %`
  title?: string
  // optional extra classes for the outer container
  className?: string
}

export const ProgressBar: React.FC<ProgressBarProps> = ({ percent, title, className }) => {
  const pct = Math.max(0, Math.min(100, Number(percent) || 0))
  const effectiveTitle = title ?? `${pct.toFixed(1)} %`

  return (
    <div
      className={`mt-0.5 flex w-full items-center gap-2 ${className || ''}`.trim()}
      title={effectiveTitle}
    >
      <div className="h-1 flex-1 rounded-full bg-surface-200 ring-1 ring-white/10 overflow-hidden">
        <div className="h-full bg-blue-500" style={{ width: pct + '%' }} />
      </div>
      <span className="text-data-xs leading-none text-primary-300 font-medium">
        {pct.toFixed(1)}%
      </span>
    </div>
  )
}

export default ProgressBar
