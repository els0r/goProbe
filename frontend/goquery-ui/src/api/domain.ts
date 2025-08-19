// domain.ts - curated UI/domain layer built on top of generated OpenAPI types
// GENERATED SCHEMA TYPES: import from './generated'
// This file contains only stable abstractions used by UI components.

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: generated file may not exist until `make types` runs
import type { components } from './generated'

// type aliases for clarity
export type ResultSchema = components['schemas']['Result']
export type RowSchema = components['schemas']['Row']
export type CountersSchema = components['schemas']['Counters']
export type AttributesSchema = components['schemas']['Attributes']
export type LabelsSchema = components['schemas']['Labels']
export type ErrorModelSchema = components['schemas']['ErrorModel']
export type SummarySchema = components['schemas']['Summary']

// Flattened flow record for table/graph usage.
export interface FlowRecord {
  sip: string
  dip: string
  dport: number | null
  proto: number | null
  bytes_in: number
  bytes_out: number
  packets_in: number
  packets_out: number
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
    packets_in,
    packets_out,
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

export interface QueryParamsUI {
  first: string
  last: string
  ifaces: string
  query: string
  // free text hosts query (separate from generic attribute list in `query`)
  query_hosts?: string
  // selected hosts resolver type from Settings; forwarded to backend as query_hosts_resolver_type
  hosts_resolver?: string
  condition?: string
  limit: number
  sort_by: 'bytes' | 'packets'
  sort_ascending: boolean
  in_only?: boolean
  out_only?: boolean
  sum?: boolean
}

export interface PagedLike<T> {
  data: T[]
  total?: number
  displayed?: number
}

export type ApiErrorCategory = 'network' | 'client' | 'parse' | 'unknown'

export interface ApiError extends Error {
  status?: number
  category: ApiErrorCategory
  body?: unknown
  problem?: ErrorModelSchema
}
