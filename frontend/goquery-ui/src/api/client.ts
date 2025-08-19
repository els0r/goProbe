import { ApiError, FlowRecord, extractFlows, QueryParamsUI, ErrorModelSchema, SummarySchema } from './domain'
import { getApiBaseUrl } from '../env'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore generated after `make types`
import type { components } from './generated'

// default timeout increased to support global-query across many hosts
const DEFAULT_TIMEOUT_MS = 300_000

type Args = components['schemas']['Args']
type ResultSchema = components['schemas']['Result']
export interface ClientConfig {
  baseUrl: string
  timeoutMs?: number
}

export class GlobalQueryClient {
  private baseUrl: string
  private timeout: number

  constructor(cfg: ClientConfig) {
    this.baseUrl = cfg.baseUrl.replace(/\/$/, '')
    this.timeout = cfg.timeoutMs ?? DEFAULT_TIMEOUT_MS
  }

  // run a query from UI params and return flattened flows
  async runQueryUI(
    params: QueryParamsUI,
    signal?: AbortSignal
  ): Promise<{ flows: FlowRecord[]; summary?: SummarySchema; hostsStatuses?: Record<string, { code?: string; message?: string }> }> {
  const args = buildArgs(params)
    const url = `${this.baseUrl}/_query`
    const controller = !signal ? new AbortController() : undefined
    const timeout = setTimeout(() => controller?.abort(), this.timeout)
    try {
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify(args),
        signal: signal ?? controller?.signal,
      })
      if (!res.ok) {
        const body = await safeJson(res)
        // attempt to detect problem+json structure
        let problem: ErrorModelSchema | undefined
        if (body && typeof body === 'object' && ('detail' in (body as any) || 'errors' in (body as any))) {
          problem = body as ErrorModelSchema
        }
        throw apiError(res.status, body, problem)
      }
  const json = (await res.json()) as ResultSchema
  const hostsStatuses = (json as any)?.hosts_statuses as Record<string, { code?: string; message?: string }> | undefined
  return { flows: extractFlows(json), summary: json?.summary as any, hostsStatuses }
    } catch (e: any) {
      if (e.name === 'AbortError') throw abortError()
      if (isApiError(e)) throw e
      throw unknownError(e)
    } finally {
      clearTimeout(timeout)
    }
  }

  // stream query results via Server-Sent Events (POST /_query/sse). The server emits named events:
  // - "partialResult": data is a Result JSON chunk; call onPartial with extracted flows
  // - "finalResult": data is the final Result JSON; call onFinal then close
  // - "error": data may include a message/host; forward to onError but do not abort
  // Optionally, a "progress" event with {done,total} may be sent.
  // Returns a disposer to close the stream.
  streamQueryUI(
    params: QueryParamsUI,
    handlers: {
      onPartial?: (flows: FlowRecord[], summary?: SummarySchema) => void
      onFinal?: (flows: FlowRecord[], summary?: SummarySchema) => void
      onError?: (err: ApiError | { message?: string; [k: string]: unknown }) => void
      onProgress?: (p: { done?: number; total?: number }) => void
      onMeta?: (meta: { hostsStatuses?: Record<string, { code?: string; message?: string }>; hostErrorCount?: number; hostOkCount?: number }) => void
    }
  ): { close: () => void } {
  const args = buildArgs(params)
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), this.timeout)
    let closed = false

    const unwrapPayload = (data: any): any => {
      if (!data) return data
      if (Array.isArray(data)) return { rows: data }
      if (typeof data !== 'object') return data
      // recursively unwrap common wrapper fields until stable
      let cur: any = data
      const unwrapOnce = (x: any): any => {
        if (!x || typeof x !== 'object') return x
        if (Array.isArray(x)) return { rows: x }
        if (x.result) return x.result
        if (x.partialResult) return x.partialResult
        if (x.finalResult) return x.finalResult
        return x
      }
      // limit iterations to avoid infinite loops
      for (let i = 0; i < 5; i++) {
        const next = unwrapOnce(cur)
        if (next === cur) break
        cur = next
      }
      // normalize common shapes to { rows }
      if (!cur.rows) {
        if (Array.isArray(cur.data)) cur = { ...cur, rows: cur.data }
        else if (Array.isArray(cur.flows)) cur = { ...cur, rows: cur.flows }
        else if (cur.rows && Array.isArray(cur.rows.data)) cur = { ...cur, rows: cur.rows.data }
      }
      return cur
    }

    const processEvent = (evt: { event?: string; data?: string }) => {
      const name = (evt.event || 'message').trim()
      const lname = name.toLowerCase()
      const raw = evt.data || ''
      const data = (() => { try { return raw ? JSON.parse(raw) : undefined } catch { return undefined } })()
    if (lname === 'partialresult' || lname === 'partial' || lname === 'message') {
        if (!data) return
        try {
          const payload: any = unwrapPayload(data)
      const flows = extractFlows(payload as ResultSchema)
      const summary = (data as any)?.summary ?? (payload as any)?.summary
      const statuses = (data as any)?.hosts_statuses ?? (payload as any)?.hosts_statuses
      if (statuses && typeof statuses === 'object') {
        let err = 0, ok = 0
        for (const k of Object.keys(statuses)) {
          const c = String((statuses as any)[k]?.code || '').toLowerCase()
          if (c === 'ok') ok++
          else err++
        }
        handlers.onMeta?.({ hostsStatuses: statuses as any, hostErrorCount: err, hostOkCount: ok })
      }
      handlers.onPartial?.(flows, summary as any)
        } catch (e) {
          handlers.onError?.(unknownError(e))
        }
        return
      }
    if (lname === 'finalresult' || lname === 'final') {
        try {
          const payload: any = unwrapPayload(data)
      const flows = extractFlows(payload as ResultSchema)
      const summary = (data as any)?.summary ?? (payload as any)?.summary
      const statuses = (data as any)?.hosts_statuses ?? (payload as any)?.hosts_statuses
      if (statuses && typeof statuses === 'object') {
        let err = 0, ok = 0
        for (const k of Object.keys(statuses)) {
          const c = String((statuses as any)[k]?.code || '').toLowerCase()
          if (c === 'ok') ok++
          else err++
        }
        handlers.onMeta?.({ hostsStatuses: statuses as any, hostErrorCount: err, hostOkCount: ok })
      }
      handlers.onFinal?.(flows, summary as any)
        } finally {
          // caller's close will abort; we also clear timeout here
          clearTimeout(timeout)
          closed = true
          controller.abort()
        }
        return
      }
      if (lname === 'progress') {
        if (data && typeof data === 'object') handlers.onProgress?.(data as any)
        return
      }
      if (lname === 'error') {
        if (data && typeof data === 'object') handlers.onError?.(data as any)
        else handlers.onError?.(unknownError(new Error('sse error event')))
        return
      }
      // heuristic fallback: if payload includes rows, treat as partial; if it signals completion, treat as final
    if (data && typeof data === 'object') {
        try {
      const payload: any = unwrapPayload(data)
          const rows = (payload as any)?.rows
          const isRowsArray = Array.isArray(rows)
          const isFinal = !!((payload as any)?.final || (data as any)?.final)
          if (isRowsArray) {
            const flows = extractFlows(payload as ResultSchema)
            if (isFinal) handlers.onFinal?.(flows, (payload as any)?.summary as any)
            else handlers.onPartial?.(flows, (payload as any)?.summary as any)
            if (isFinal) {
              clearTimeout(timeout)
              closed = true
              controller.abort()
            }
            return
          }
        } catch (e) {
          // fall through to ignore
        }
      }
      // ignore other events
    }

    // kick off POST fetch that returns text/event-stream
    ;(async () => {
      try {
        const res = await fetch(`${this.baseUrl}/_query/sse`, {
          method: 'POST',
          headers: {
            'accept': 'text/event-stream',
            'content-type': 'application/json',
          },
          body: JSON.stringify(args),
          signal: controller.signal,
        })
  if (!res.ok) {
          const body = await safeJson(res)
          let problem: ErrorModelSchema | undefined
          if (body && typeof body === 'object' && ('detail' in (body as any) || 'errors' in (body as any))) {
            problem = body as ErrorModelSchema
          }
          throw apiError(res.status, body, problem)
        }
        const reader = res.body?.getReader()
        if (!reader) throw unknownError(new Error('no response body'))
        const decoder = new TextDecoder('utf-8')
        let buf = ''
        while (true) {
          const { value, done } = await reader.read()
          if (done) break
          buf += decoder.decode(value, { stream: true })
          // normalize newlines and parse complete events (\n\n delimiter)
          buf = buf.replace(/\r\n/g, '\n')
          let idx
          while ((idx = buf.indexOf('\n\n')) >= 0) {
            const rawEvt = buf.slice(0, idx)
            buf = buf.slice(idx + 2)
            // parse one event block
            const evt: { event?: string; data?: string } = {}
            const lines = rawEvt.split('\n')
            for (const ln of lines) {
              if (!ln) continue
              if (ln.startsWith(':')) continue // comment
              const m = ln.match(/^(\w+):\s?(.*)$/)
              if (!m) continue
              const k = m[1]
              const v = m[2]
              if (k === 'event') evt.event = v
              else if (k === 'data') evt.data = (evt.data ? evt.data + '\n' : '') + v
            }
            processEvent(evt)
            if (closed) return
          }
        }
        // stream ended: process any trailing, unterminated event block
        if (buf && buf.trim().length > 0) {
          const rawEvt = buf.replace(/\r\n/g, '\n')
          const evt: { event?: string; data?: string } = {}
          const lines = rawEvt.split('\n')
          for (const ln of lines) {
            if (!ln) continue
            if (ln.startsWith(':')) continue
            const m = ln.match(/^(\w+):\s?(.*)$/)
            if (!m) continue
            const k = m[1]
            const v = m[2]
            if (k === 'event') evt.event = v
            else if (k === 'data') evt.data = (evt.data ? evt.data + '\n' : '') + v
          }
          if (evt.event || evt.data) {
            processEvent(evt)
            if (closed) return
          }
        }
      } catch (e: any) {
        if (e?.name === 'AbortError') {
          // timed out or manual close; report as abort to onError so UI can reflect non-fatal end
          handlers.onError?.(abortError())
        } else if (!closed) {
          handlers.onError?.(isApiError(e) ? e : unknownError(e))
        }
      } finally {
        clearTimeout(timeout)
      }
    })()

    return {
      close: () => {
        try { clearTimeout(timeout); controller.abort() } catch {}
        closed = true
      },
    }
  }
}

let _defaultClient: GlobalQueryClient | undefined
export function getGlobalQueryClient(): GlobalQueryClient {
  if (_defaultClient) return _defaultClient
  const base = getApiBaseUrl()
  _defaultClient = new GlobalQueryClient({ baseUrl: base })
  return _defaultClient
}

// allow UI to change backend dynamically
export function setGlobalQueryBaseUrl(baseUrl: string, timeoutMs?: number): GlobalQueryClient {
  const base = (baseUrl || '').replace(/\/$/, '')
  _defaultClient = new GlobalQueryClient({ baseUrl: base || getApiBaseUrl(), timeoutMs })
  return _defaultClient
}

function buildArgs(p: QueryParamsUI): Args {
  return {
    query: p.query,
  query_hosts: p.query_hosts,
    ifaces: p.ifaces,
    first: p.first,
    last: p.last,
    condition: p.condition,
    in: p.in_only || undefined,
    out: p.out_only || undefined,
    sum: p.sum || undefined,
    num_results: p.limit,
    sort_by: p.sort_by,
    sort_ascending: p.sort_ascending,
  // forward UI-selected hosts resolver to backend schema field
  query_hosts_resolver_type: p.hosts_resolver || undefined,
    format: 'json',
  }
}

function apiError(status: number, body: unknown, problem?: ErrorModelSchema): ApiError {
  return {
    name: 'ApiError',
    message: `api request failed: status=${status}`,
    status,
    category: status >= 500 ? 'network' : 'client',
    body,
  problem,
  }
}

function abortError(): ApiError {
  return {
    name: 'AbortError',
    message: 'request aborted',
    category: 'network',
  }
}

function unknownError(err: unknown): ApiError {
  return {
    name: 'UnknownError',
    message: 'unknown error',
    category: 'unknown',
    body: err,
  }
}

function isApiError(e: any): e is ApiError {
  return e && typeof e === 'object' && 'category' in e
}

async function safeJson(res: Response): Promise<unknown> {
  try { return await res.json() } catch { return undefined }
}

// build query string from Args for GET /sse
function toQueryString(args: Args): string {
  const qp = new URLSearchParams()
  const push = (k: string, v: unknown) => {
    if (v === undefined || v === null || v === '') return
    qp.set(k, String(v))
  }
  push('query', (args as any).query)
  push('query_hosts', (args as any).query_hosts)
  push('ifaces', (args as any).ifaces)
  push('first', (args as any).first)
  push('last', (args as any).last)
  push('condition', (args as any).condition)
  push('in', (args as any).in)
  push('out', (args as any).out)
  push('sum', (args as any).sum)
  push('num_results', (args as any).num_results)
  push('sort_by', (args as any).sort_by)
  push('sort_ascending', (args as any).sort_ascending)
  push('query_hosts_resolver_type', (args as any).query_hosts_resolver_type)
  push('format', 'json')
  return qp.toString()
}
