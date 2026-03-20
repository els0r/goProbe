import React from 'react'

export interface ProgressBarProps {
  // linear percentage in [0,100]
  percent: number
  // optional title attribute override; defaults to `${percent.toFixed(1)} %`
  title?: string
  // optional extra classes for the outer container
  className?: string
}

// map a linear share in [0,1] to a non-linear width in [0,100] using log10
// this emphasizes small values while keeping 0 -> 0 and 1 -> 100
function nonLinearWidthPct(share: number): number {
  if (!isFinite(share) || share <= 0) return 0
  if (share >= 1) return 100
  return Math.log10(1 + 9 * share) * 100
}

function clamp01(v: number): number {
  if (!isFinite(v)) return 0
  if (v < 0) return 0
  if (v > 1) return 1
  return v
}

export const ProgressBar: React.FC<ProgressBarProps> = ({ percent, title, className }) => {
  const pctLinear = Math.max(0, Math.min(100, Number(percent) || 0))
  const share = clamp01(pctLinear / 100)
  const pctWidth = nonLinearWidthPct(share)
  const effectiveTitle = title ?? `${pctLinear.toFixed(1)} %`

  return (
    <div
      className={`mt-0.5 flex w-full items-center gap-2 ${className || ''}`.trim()}
      title={effectiveTitle}
    >
      <div className="h-1 flex-1 rounded-full bg-surface-200 ring-1 ring-white/10 overflow-hidden">
        <div className="h-full bg-blue-500" style={{ width: pctWidth + '%' }} />
      </div>
      <span className="text-[10px] leading-none text-primary-300 font-medium">
        {pctLinear.toFixed(1)}%
      </span>
    </div>
  )
}

export default ProgressBar
