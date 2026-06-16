// Grouping and scaling over sets of Flows: the aggregates the details panels
// and table bars derive from a Run's (or Detail Run's) result rows.
import { FlowRecord } from './record'

export interface ServiceGroup {
  proto: number | null
  dport: number | null
  inB: number
  outB: number
  inP: number
  outP: number
}

export interface IfaceGroup {
  iface: string
  inB: number
  outB: number
  inP: number
  outP: number
}

export interface Totals {
  inB: number
  outB: number
  inP: number
  outP: number
}

export function sumTotals(rows: FlowRecord[]): Totals {
  return rows.reduce(
    (acc, r) => {
      acc.inB += r.bytes_in
      acc.outB += r.bytes_out
      acc.inP += r.packets_in
      acc.outP += r.packets_out
      return acc
    },
    { inB: 0, outB: 0, inP: 0, outP: 0 }
  )
}

export function groupByService(rows: FlowRecord[]): ServiceGroup[] {
  const m = new Map<string, ServiceGroup>()
  for (const r of rows) {
    const key = `${r.proto ?? 'na'}|${r.dport ?? 'na'}`
    const g = m.get(key) || {
      proto: r.proto ?? null,
      dport: r.dport ?? null,
      inB: 0,
      outB: 0,
      inP: 0,
      outP: 0,
    }
    g.inB += r.bytes_in
    g.outB += r.bytes_out
    g.inP += r.packets_in
    g.outP += r.packets_out
    m.set(key, g)
  }
  return Array.from(m.values()).sort((a, b) => b.inB + b.outB - (a.inB + a.outB))
}

export function groupByIface(rows: FlowRecord[]): IfaceGroup[] {
  const m = new Map<string, IfaceGroup>()
  for (const r of rows) {
    const key = r.iface || '(iface)'
    const g = m.get(key) || { iface: key, inB: 0, outB: 0, inP: 0, outP: 0 }
    g.inB += r.bytes_in
    g.outB += r.bytes_out
    g.inP += r.packets_in
    g.outP += r.packets_out
    m.set(key, g)
  }
  return Array.from(m.values()).sort((a, b) => b.inB + b.outB - (a.inB + a.outB))
}

/**
 * Shared global max over every in *and* out magnitude across `rows` — the
 * single scale reference each diverging bar in one table normalizes against,
 * so bars stay comparable both within a row (in vs out) and down the column.
 *
 * Recomputed live as streaming rows arrive; returns `0` for an empty set.
 */
export function inOutScaleMax(
  rows: ReadonlyArray<FlowRecord>,
  inKey: 'bytes_in' | 'packets_in',
  outKey: 'bytes_out' | 'packets_out',
): number {
  let max = 0
  for (const r of rows) {
    const i = r[inKey] || 0
    const o = r[outKey] || 0
    if (i > max) max = i
    if (o > max) max = o
  }
  return max
}
