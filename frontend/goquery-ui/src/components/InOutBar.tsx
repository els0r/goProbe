import React from 'react'
import { inOutBarGeometry } from '../utils/inOutBar'

export interface InOutBarProps {
  /** inbound magnitude (left half) — bytes_in or packets_in */
  inValue: number
  /** outbound magnitude (right half) — bytes_out or packets_out */
  outValue: number
  /** shared global max across the table, sets the common scale */
  scaleMax: number
  /** unidirectional flow — paints the (single) drawn half red instead of blue */
  unidirectional?: boolean
  /** print the in/out figures that informed the bar's shape, flanking it */
  showValues?: boolean
  /** formats the flanking labels — humanBytes for byte bars, humanPackets for packet bars */
  format?: (n: number) => string
}

/**
 * One centered-axis diverging in/out gauge: the inbound half grows left and the
 * outbound half grows right, both square-root–scaled against a shared column
 * max so bars stay comparable down the column and within a row. The total lives
 * in its own right-aligned column (see TableView) — keeping this cell purely
 * graphical lets the bars stack into an uninterrupted vertical reading flow.
 * Exact in/out/total counts surface in the hover tooltip.
 */
export const InOutBar: React.FC<InOutBarProps> = ({
  inValue,
  outValue,
  scaleMax,
  unidirectional = false,
  showValues = false,
  format,
}) => {
  const { inFrac, outFrac } = inOutBarGeometry(inValue, outValue, scaleMax)
  const total = (inValue || 0) + (outValue || 0)
  const tooltip =
    `in: ${(inValue || 0).toLocaleString()}` +
    ` • out: ${(outValue || 0).toLocaleString()}` +
    ` • total: ${total.toLocaleString()}`
  // bright vs deep blue for clear in/out separation; red flags a one-way flow
  const inColor = unidirectional ? 'bg-red-400/80' : 'bg-primary-300'
  const outColor = unidirectional ? 'bg-red-400/80' : 'bg-primary-500'

  const bar = (
    <div title={tooltip} className="relative h-2 w-40">
      <div className="flex h-full w-full">
        <div className="flex w-1/2 justify-end pr-px">
          <div
            className={`h-full rounded-sm ${inColor}`}
            style={{ width: `${inFrac * 100}%` }}
          />
        </div>
        <div className="flex w-1/2 justify-start pl-px">
          <div
            className={`h-full rounded-sm ${outColor}`}
            style={{ width: `${outFrac * 100}%` }}
          />
        </div>
      </div>
    </div>
  )

  if (!showValues || !format) {
    return <div className="mx-auto h-2 w-40">{bar}</div>
  }

  // fixed-width gutters flank the bar so it stays centred and aligned down the
  // column regardless of label length; in reads right→bar, out reads bar→left.
  // data-xs/leading-none keeps the label height close to the 8px bar.
  const label = 'w-14 shrink-0 whitespace-nowrap tabular-nums text-data-xs leading-none text-gray-500'
  return (
    <div className="mx-auto flex items-center justify-center gap-1.5">
      <span className={`${label} text-right`}>{format(inValue || 0)}</span>
      {bar}
      <span className={`${label} text-left`}>{format(outValue || 0)}</span>
    </div>
  )
}

export default InOutBar
