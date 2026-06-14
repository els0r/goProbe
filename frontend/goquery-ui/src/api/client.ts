import { FlowRecord, extractFlows } from '../flows'
import { SummarySchema } from './domain'
import type { QueryParamsUI } from '../query/params'
import {
  ApiError,
  apiError,
  abortError,
  unknownError,
  isApiError,
  safeJson,
  extractProblem,
} from './errors'
import { SSEParser } from './sse'
import { interpretSSEEvent, SSEOutcome } from './sseEvents'
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

  private bustUrl(path: string): string {
    try {
      const url = new URL(path, this.baseUrl)
      url.searchParams.set('_ts', String(Date.now()))
      return url.toString()
    } catch {
      // Fallback: naive concat
      const sep = path.includes('?') ? '&' : '?'
      return `${this.baseUrl.replace(/\/$/, '')}${path}${sep}_ts=${Date.now()}`
    }
  }

  private async delay(ms: number) {
    return new Promise((res) => setTimeout(res, ms))
  }

  private isLikelyConnReset(err: any): boolean {
    try {
      const name = String(err?.name || '')
      const msg = String(err?.message || '')
      const bodyMsg = String((err?.body as any)?.message || '')
      const s = (msg + ' ' + bodyMsg).toLowerCase()
      // heuristics for Chromium/WebKit/Gecko
      return (
        name === 'TypeError' ||
        s.includes('err_connection_reset') ||
        s.includes('networkerror') ||
        s.includes('network error')
      )
    } catch {
      return false
    }
  }

  // validate a query without running it. Returns void on success (HTTP 204), throws ApiError on failure
  async validateQueryUI(params: QueryParamsUI, signal?: AbortSignal): Promise<void> {
    const args = buildArgs(params)
    const url = this.bustUrl('/_query/validate')
    for (let attempt = 0; attempt < 2; attempt++) {
      const controller = !signal ? new AbortController() : undefined
      const timeout = setTimeout(() => controller?.abort(), this.timeout)
      try {
        const res = await fetch(url, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify(args),
          signal: signal ?? controller?.signal,
          cache: 'no-store',
        } as RequestInit)
        if (res.status === 204) return
        if (!res.ok) {
          const body = await safeJson(res)
          throw apiError(res.status, body, extractProblem(body))
        }
        // treat as success
        return
      } catch (e: any) {
        if (e?.name === 'AbortError') throw abortError()
        if (attempt === 0 && this.isLikelyConnReset(e)) {
          await this.delay(150)
          continue
        }
        if (isApiError(e)) throw e
        throw unknownError(e)
      } finally {
        clearTimeout(timeout)
      }
    }
    // should not reach here; both attempts failed with thrown errors above
    throw unknownError(new Error('validate failed'))
  }

  // run a query from UI params and return flattened flows
  async runQueryUI(
    params: QueryParamsUI,
    signal?: AbortSignal
  ): Promise<{
    flows: FlowRecord[]
    summary?: SummarySchema
    hostsStatuses?: Record<string, { code?: string; message?: string }>
  }> {
    const args = buildArgs(params)
    const url = this.bustUrl('/_query')
    for (let attempt = 0; attempt < 2; attempt++) {
      const controller = !signal ? new AbortController() : undefined
      const timeout = setTimeout(() => controller?.abort(), this.timeout)
      try {
        const res = await fetch(url, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify(args),
          signal: signal ?? controller?.signal,
          cache: 'no-store',
        } as RequestInit)
        if (!res.ok) {
          const body = await safeJson(res)
          throw apiError(res.status, body, extractProblem(body))
        }
        const json = (await res.json()) as ResultSchema
        const hostsStatuses = (json as any)?.hosts_statuses as
          | Record<string, { code?: string; message?: string }>
          | undefined
        return {
          flows: extractFlows(json),
          summary: json?.summary as any,
          hostsStatuses,
        }
      } catch (e: any) {
        if (e?.name === 'AbortError') throw abortError()
        if (attempt === 0 && this.isLikelyConnReset(e)) {
          await this.delay(150)
          continue
        }
        if (isApiError(e)) throw e
        throw unknownError(e)
      } finally {
        clearTimeout(timeout)
      }
    }
    // should not reach here; both attempts failed with thrown errors above
    throw unknownError(new Error('request failed'))
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
      onMeta?: (meta: {
        hostsStatuses?: Record<string, { code?: string; message?: string }>
        hostErrorCount?: number
        hostOkCount?: number
      }) => void
    }
  ): { close: () => void } {
    const args = buildArgs(params)
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), this.timeout)
    let closed = false
    let readerRef: ReadableStreamDefaultReader<Uint8Array> | undefined

    // Apply one interpreted outcome to the handlers. Returns true when the
    // stream is complete (final event), which also performs the single
    // close/abort that ends the read loop.
    const dispatch = (o: SSEOutcome): boolean => {
      switch (o.kind) {
        case 'partial':
          if (o.meta) handlers.onMeta?.(o.meta)
          handlers.onPartial?.(o.flows, o.summary)
          return false
        case 'final':
          if (o.meta) handlers.onMeta?.(o.meta)
          handlers.onFinal?.(o.flows, o.summary)
          clearTimeout(timeout)
          closed = true
          controller.abort()
          return true
        case 'error':
          handlers.onError?.(o.error)
          return false
        case 'progress':
          handlers.onProgress?.(o.progress)
          return false
        case 'ignore':
          return false
      }
    }

    const connect = async () =>
      await fetch(this.bustUrl('/_query/sse'), {
        method: 'POST',
        headers: {
          accept: 'text/event-stream',
          'content-type': 'application/json',
        },
        body: JSON.stringify(args),
        signal: controller.signal,
        cache: 'no-store',
      } as RequestInit)

    // Read an SSE response to completion, feeding bytes through the parser and
    // dispatching each interpreted event. Returns when the stream ends or a
    // final/close stops it.
    const drainStream = async (reader: ReadableStreamDefaultReader<Uint8Array>) => {
      readerRef = reader
      const parser = new SSEParser()
      while (true) {
        const { value, done } = await reader.read()
        if (done) break
        for (const evt of parser.push(value)) {
          if (dispatch(interpretSSEEvent(evt))) return
          if (closed) return
        }
      }
      // process any trailing, unterminated event block
      for (const evt of parser.flush()) {
        if (dispatch(interpretSSEEvent(evt))) return
        if (closed) return
      }
    }

    // kick off POST fetch that returns text/event-stream
    const openAndDrain = async () => {
      const res = await connect()
      if (!res.ok) {
        const body = await safeJson(res)
        throw apiError(res.status, body, extractProblem(body))
      }
      const reader = res.body?.getReader()
      if (!reader) throw unknownError(new Error('no response body'))
      await drainStream(reader)
    }

    ;(async () => {
      let triedReconnect = false
      try {
        await openAndDrain()
      } catch (e: any) {
        // if the server closed the connection aggressively, attempt one quick reconnect
        if (!closed && !triedReconnect && this.isLikelyConnReset(e)) {
          triedReconnect = true
          try {
            await this.delay(150)
            await openAndDrain()
          } catch (re) {
            if (!closed) handlers.onError?.(isApiError(re) ? (re as any) : unknownError(re))
          }
        } else if (e?.name === 'AbortError') {
          if (!closed) handlers.onError?.(abortError())
        } else if (!closed) {
          handlers.onError?.(isApiError(e) ? e : unknownError(e))
        }
      } finally {
        clearTimeout(timeout)
      }
    })()

    return {
      close: () => {
        try {
          clearTimeout(timeout)
          // mark closed first to prevent catch handler from reporting AbortError
          closed = true
          // proactively cancel the reader to close the stream
          try {
            readerRef?.cancel()
          } catch {}
          controller.abort()
        } catch {}
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
  _defaultClient = new GlobalQueryClient({
    baseUrl: base || getApiBaseUrl(),
    timeoutMs,
  })
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

