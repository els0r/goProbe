// The Flow concept (CONTEXT.md): one observed traffic relationship — a
// source/destination/port/protocol tuple with its in/out byte and packet
// counters — flattened from the wire Row into a stable, UI-friendly shape.
import type {
  RowSchema,
  AttributesSchema,
  CountersSchema,
  LabelsSchema,
  ResultSchema,
} from '../api/domain'

// Flattened flow record for table/graph usage.
export interface FlowRecord {
  sip: string
  dip: string
  dport: number | null
  proto: number | null
  bytes_in: number
  bytes_out: number
  // in+out volume of this Flow; derived once here (like `bidirectional`) so the
  // ~20 call sites that need it read a field instead of re-summing. Mirrors the
  // Run-aggregate naming in ResultTotals (flows/totals.ts).
  bytes_total: number
  packets_in: number
  packets_out: number
  packets_total: number
  host?: string
  host_id?: string
  iface?: string
  interval_end?: string
  bidirectional: boolean
  _raw: RowSchema
}

export function flattenRow(row: RowSchema): FlowRecord {
  const a = row.attributes || ({} as AttributesSchema)
  const c = row.counters || ({} as CountersSchema)
  const l = row.labels || ({} as LabelsSchema)
  const bytes_in = (c as any).br ?? 0
  const bytes_out = (c as any).bs ?? 0
  const packets_in = (c as any).pr ?? 0
  const packets_out = (c as any).ps ?? 0
  return {
    sip: (a as any).sip || '',
    dip: (a as any).dip || '',
    dport: (a as any).dport ?? null,
    proto: (a as any).proto ?? null,
    bytes_in,
    bytes_out,
    bytes_total: bytes_in + bytes_out,
    packets_in,
    packets_out,
    packets_total: packets_in + packets_out,
    host: (l as any).host,
    host_id: (l as any).host_id,
    iface: (l as any).iface,
    interval_end: (l as any).timestamp,
    bidirectional: bytes_in > 0 && bytes_out > 0 && packets_in > 0 && packets_out > 0,
    _raw: row,
  }
}

export function extractFlows(result: ResultSchema | undefined | null): FlowRecord[] {
  if (!result?.rows) return []
  const rows: RowSchema[] = (result.rows ?? []) as unknown as RowSchema[]
  return rows.map((r: RowSchema) => flattenRow(r))
}
