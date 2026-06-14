// Run-level aggregates over Flows (CONTEXT.md: Run Total, Run Share).
import type { SummarySchema, CountersSchema } from '../api/domain'

// Aggregate traffic volume of a Run, decoded from the Summary's wire counters.
// Same naming as FlowRecord (bytes_in/out/total, packets_in/out/total); the
// in+out total is derived once here instead of at every call site.
export interface ResultTotals {
  bytes_in: number
  bytes_out: number
  bytes_total: number
  packets_in: number
  packets_out: number
  packets_total: number
}

// Decodes Summary.totals (br/bs/pr/ps) into domain-named aggregate totals. The
// br/bs/pr/ps wire shape is confined to this function. Absent summary or counters
// yield zeros (mirrors extractFlows), so callers never guard.
export function resultTotals(summary: SummarySchema | undefined): ResultTotals {
  const c = (summary?.totals ?? {}) as CountersSchema
  const bytes_in = c.br ?? 0
  const bytes_out = c.bs ?? 0
  const packets_in = c.pr ?? 0
  const packets_out = c.ps ?? 0
  return {
    bytes_in,
    bytes_out,
    bytes_total: bytes_in + bytes_out,
    packets_in,
    packets_out,
    packets_total: packets_in + packets_out,
  }
}

/**
 * One Flow's volume (`part`) as a percentage of the Run Total (`runTotal`).
 *
 * Owns the clamp to `[0, 100]` and the `runTotal > 0` guard so call sites carry
 * neither: a zero/absent Run Total yields `0`, and a transient streaming
 * overshoot can never exceed `100`. Returns a number — display precision (e.g.
 * `.toFixed(1)` in the table, `.toFixed(2)` in the export) stays at the call
 * site, by intent. NOT the temporal heatmap's peak-bucket ratio nor the
 * diverging-bar sqrt fill — those are not shares of the Run (CONTEXT.md).
 */
export function runSharePct(part: number, runTotal: number): number {
  if (!(runTotal > 0)) return 0
  const pct = (part / runTotal) * 100
  return pct < 0 ? 0 : pct > 100 ? 100 : pct
}
