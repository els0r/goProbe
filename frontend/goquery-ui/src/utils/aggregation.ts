import { FlowRecord } from '../api/domain'

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
