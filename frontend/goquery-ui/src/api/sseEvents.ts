// SSE event -> domain interpreter. Pure: it maps one parsed SSEEvent to an
// outcome describing what the caller should do, without applying any side
// effects (no handler calls, no aborts). The transport (client.ts) owns those.
//
// This is where the "Host Error vs fatal" domain rule lives (CONTEXT.md): an
// `error` *event* is a non-fatal Host Error forwarded to the caller; a dropped
// connection is a fatal Run failure handled by the transport, not here.
import { extractFlows, FlowRecord } from '../flows'
import { ResultSchema, SummarySchema } from './domain'
import { ApiError, unknownError } from './errors'
import { SSEEvent } from './sse'

export interface HostStatusCounts {
  hostsStatuses: Record<string, { code?: string; message?: string }>
  hostOkCount: number
  hostErrorCount: number
}

export type SSEOutcome =
  | { kind: 'partial'; flows: FlowRecord[]; summary?: SummarySchema; meta?: HostStatusCounts }
  | { kind: 'final'; flows: FlowRecord[]; summary?: SummarySchema; meta?: HostStatusCounts }
  | { kind: 'error'; error: ApiError | { message?: string; [k: string]: unknown } }
  | { kind: 'progress'; progress: { done?: number; total?: number } }
  | { kind: 'ignore' }

// Recursively unwrap common server envelope shapes down to a { rows } payload.
// Iteration is bounded to avoid infinite loops on self-referential wrappers.
export function unwrapPayload(data: any): any {
  if (!data) return data
  if (Array.isArray(data)) return { rows: data }
  if (typeof data !== 'object') return data
  let cur: any = data
  const unwrapOnce = (x: any): any => {
    if (!x || typeof x !== 'object') return x
    if (Array.isArray(x)) return { rows: x }
    if (x.result) return x.result
    if (x.partialResult) return x.partialResult
    if (x.finalResult) return x.finalResult
    return x
  }
  for (let i = 0; i < 5; i++) {
    const next = unwrapOnce(cur)
    if (next === cur) break
    cur = next
  }
  if (!cur.rows) {
    if (Array.isArray(cur.data)) cur = { ...cur, rows: cur.data }
    else if (Array.isArray(cur.flows)) cur = { ...cur, rows: cur.flows }
    else if (cur.rows && Array.isArray(cur.rows.data)) cur = { ...cur, rows: cur.rows.data }
  }
  return cur
}

// Tally per-host statuses into ok/error counts. Returns undefined when no
// status map is present so the caller can skip the onMeta notification.
export function countHostStatuses(statuses: any): HostStatusCounts | undefined {
  if (!statuses || typeof statuses !== 'object') return undefined
  let err = 0
  let ok = 0
  for (const k of Object.keys(statuses)) {
    const c = String((statuses as any)[k]?.code || '').toLowerCase()
    if (c === 'ok') ok++
    else err++
  }
  return { hostsStatuses: statuses, hostOkCount: ok, hostErrorCount: err }
}

function parseData(raw: string): any {
  try {
    return raw ? JSON.parse(raw) : undefined
  } catch {
    return undefined
  }
}

export function interpretSSEEvent(evt: SSEEvent): SSEOutcome {
  const lname = (evt.event || 'message').trim().toLowerCase()
  const data = parseData(evt.data || '')

  if (lname === 'partialresult' || lname === 'partial' || lname === 'message') {
    if (!data) return { kind: 'ignore' }
    try {
      const payload: any = unwrapPayload(data)
      const flows = extractFlows(payload as ResultSchema)
      const summary = (data as any)?.summary ?? (payload as any)?.summary
      const statuses = (data as any)?.hosts_statuses ?? (payload as any)?.hosts_statuses
      return { kind: 'partial', flows, summary: summary as any, meta: countHostStatuses(statuses) }
    } catch (e) {
      return { kind: 'error', error: unknownError(e) }
    }
  }

  if (lname === 'finalresult' || lname === 'final') {
    const payload: any = unwrapPayload(data)
    const flows = extractFlows(payload as ResultSchema)
    const summary = (data as any)?.summary ?? (payload as any)?.summary
    const statuses = (data as any)?.hosts_statuses ?? (payload as any)?.hosts_statuses
    return { kind: 'final', flows, summary: summary as any, meta: countHostStatuses(statuses) }
  }

  if (lname === 'progress') {
    if (data && typeof data === 'object') return { kind: 'progress', progress: data as any }
    return { kind: 'ignore' }
  }

  if (lname === 'error') {
    if (data && typeof data === 'object') return { kind: 'error', error: data as any }
    return { kind: 'error', error: unknownError(new Error('sse error event')) }
  }

  // heuristic fallback for unnamed events: rows present => partial/final
  if (data && typeof data === 'object') {
    try {
      const payload: any = unwrapPayload(data)
      const rows = (payload as any)?.rows
      if (Array.isArray(rows)) {
        const flows = extractFlows(payload as ResultSchema)
        const summary = (payload as any)?.summary
        const isFinal = !!((payload as any)?.final || (data as any)?.final)
        return isFinal
          ? { kind: 'final', flows, summary: summary as any }
          : { kind: 'partial', flows, summary: summary as any }
      }
    } catch {
      // fall through to ignore
    }
  }
  return { kind: 'ignore' }
}
