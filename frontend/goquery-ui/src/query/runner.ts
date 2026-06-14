// QueryRunner — framework-free state machine orchestrating query Runs.
// See CONTEXT.md (Run, Validation, Host Error) and docs/adr/0001-framework-free-query-runner.md.
import { FlowRecord } from '../flows'
import { SummarySchema } from '../api/domain'
import { QueryParamsUI } from './params'
import { ApiError, isApiError, toApiError } from '../api/errors'

export type RunPhase = 'idle' | 'validating' | 'running' | 'done' | 'error'

export type HostsStatuses = Record<string, { code?: string; message?: string }>

export interface HostError {
  host?: string
  message?: string
}

export interface RunnerSnapshot {
  phase: RunPhase
  // true while rows reflect an in-progress (streamed) result
  partial: boolean
  rows: FlowRecord[]
  summary?: SummarySchema
  progress: { done?: number; total?: number }
  hostsStatuses: HostsStatuses
  hostOkCount: number
  hostErrorCount: number
  // from preflight or standalone validation; never affects phase
  validationError: ApiError | null
  // fatal Run failure, including stream connection loss
  runError: ApiError | null
  // per-host failures of a fanned-out Run; non-fatal, accumulated
  hostErrors: HostError[]
}

export interface StreamHandlers {
  onPartial?: (flows: FlowRecord[], summary?: SummarySchema) => void
  onFinal?: (flows: FlowRecord[], summary?: SummarySchema) => void
  onError?: (err: ApiError | { message?: string; [k: string]: unknown }) => void
  onProgress?: (p: { done?: number; total?: number }) => void
  onMeta?: (meta: {
    hostsStatuses?: HostsStatuses
    hostErrorCount?: number
    hostOkCount?: number
  }) => void
}

// Consumer-side contract: the minimal client surface the runner needs.
// GlobalQueryClient satisfies it structurally.
export interface QueryClient {
  validateQueryUI(params: QueryParamsUI, signal?: AbortSignal): Promise<void>
  runQueryUI(
    params: QueryParamsUI,
    signal?: AbortSignal
  ): Promise<{
    flows: FlowRecord[]
    summary?: SummarySchema
    hostsStatuses?: HostsStatuses
  }>
  streamQueryUI(params: QueryParamsUI, handlers: StreamHandlers): { close: () => void }
}

const INITIAL: RunnerSnapshot = {
  phase: 'idle',
  partial: false,
  rows: [],
  summary: undefined,
  progress: {},
  hostsStatuses: {},
  hostOkCount: 0,
  hostErrorCount: 0,
  validationError: null,
  runError: null,
  hostErrors: [],
}

function countHostStatuses(statuses: HostsStatuses): { ok: number; error: number } {
  let ok = 0
  let error = 0
  for (const k of Object.keys(statuses)) {
    if (String(statuses[k]?.code || '').toLowerCase() === 'ok') ok++
    else error++
  }
  return { ok, error }
}

export class QueryRunner {
  // Snapshot contract: reference-stable between transitions, replaced atomically on
  // each transition, and listeners are only notified after the new snapshot is set.
  private snapshot: RunnerSnapshot = INITIAL
  private listeners = new Set<() => void>()
  // Every run() and cancel() bumps the generation; async continuations of a
  // superseded Run compare against it and become inert.
  private generation = 0
  private streamCloser: { close: () => void } | null = null
  private runAbort: AbortController | null = null
  private validateAbort: AbortController | null = null

  constructor(private getClient: () => QueryClient) {}

  getSnapshot(): RunnerSnapshot {
    return this.snapshot
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  // Starts a Run: preflight validation, then streamed or plain execution.
  // Supersedes any Run in flight. Outcome is observed via the snapshot only.
  run(params: QueryParamsUI, opts: { stream: boolean }): void {
    const gen = ++this.generation
    this.disposeInflight()
    // resolve the client once per Run; a backend switch applies from the next Run
    const client = this.getClient()
    // rows clear at Run start (explicit policy; keeps pre-runner behaviour).
    // validationError is kept until the preflight verdict replaces it.
    this.transition({
      phase: 'validating',
      partial: false,
      rows: [],
      summary: undefined,
      progress: {},
      hostsStatuses: {},
      hostOkCount: 0,
      hostErrorCount: 0,
      runError: null,
      hostErrors: [],
    })
    void (async () => {
      const ok = await this.preflight(client, params, gen)
      if (!ok) return
      this.transition({ phase: 'running', validationError: null })
      if (opts.stream) this.startStream(client, params, gen)
      else await this.startRequest(client, params, gen)
    })()
  }

  // Standalone Validation (editor feedback). Supersedes any in-flight Validation
  // but never touches the Run phase.
  async validate(params: QueryParamsUI): Promise<boolean> {
    this.validateAbort?.abort()
    const ctrl = new AbortController()
    this.validateAbort = ctrl
    try {
      await this.getClient().validateQueryUI(params, ctrl.signal)
      if (this.validateAbort !== ctrl) return false // superseded
      this.validateAbort = null
      this.transition({ validationError: null })
      return true
    } catch (e) {
      if (this.validateAbort !== ctrl) return false
      this.validateAbort = null
      this.transition({ validationError: toApiError(e) })
      return false
    }
  }

  // Cancels whatever is in flight and returns to idle. Rows and summary are
  // kept; transient Run state and errors are cleared. Synchronous.
  cancel(): void {
    this.generation++
    this.disposeInflight()
    this.transition({
      phase: 'idle',
      partial: false,
      progress: {},
      hostsStatuses: {},
      hostOkCount: 0,
      hostErrorCount: 0,
      validationError: null,
      runError: null,
      hostErrors: [],
    })
  }

  private transition(patch: Partial<RunnerSnapshot>) {
    this.snapshot = { ...this.snapshot, ...patch }
    for (const l of [...this.listeners]) l()
  }

  private disposeInflight() {
    if (this.streamCloser) {
      try {
        this.streamCloser.close()
      } catch {}
      this.streamCloser = null
    }
    if (this.runAbort) {
      try {
        this.runAbort.abort()
      } catch {}
      this.runAbort = null
    }
    if (this.validateAbort) {
      try {
        this.validateAbort.abort()
      } catch {}
      this.validateAbort = null
    }
  }

  // Preflight validation of a Run. On failure the Run ends: phase returns to
  // idle with validationError set (unless the preflight itself was superseded).
  private async preflight(
    client: QueryClient,
    params: QueryParamsUI,
    gen: number
  ): Promise<boolean> {
    this.validateAbort?.abort()
    const ctrl = new AbortController()
    this.validateAbort = ctrl
    try {
      await client.validateQueryUI(params, ctrl.signal)
      if (gen !== this.generation) return false
      if (this.validateAbort === ctrl) this.validateAbort = null
      return true
    } catch (e) {
      if (gen !== this.generation) return false
      if (this.validateAbort === ctrl) this.validateAbort = null
      if ((e as { name?: string })?.name === 'AbortError') {
        // superseded by a newer standalone Validation; its verdict will land instead
        this.transition({ phase: 'idle' })
      } else {
        this.transition({ phase: 'idle', validationError: toApiError(e) })
      }
      return false
    }
  }

  private async startRequest(client: QueryClient, params: QueryParamsUI, gen: number) {
    const ctrl = new AbortController()
    this.runAbort = ctrl
    try {
      const data = await client.runQueryUI(params, ctrl.signal)
      if (gen !== this.generation) return
      this.runAbort = null
      const statuses = data.hostsStatuses ?? {}
      const counts = countHostStatuses(statuses)
      this.transition({
        phase: 'done',
        partial: false,
        rows: data.flows,
        summary: data.summary,
        hostsStatuses: statuses,
        hostOkCount: counts.ok,
        hostErrorCount: counts.error,
      })
    } catch (e) {
      if (gen !== this.generation) return
      this.runAbort = null
      this.transition({ phase: 'error', partial: false, runError: toApiError(e) })
    }
  }

  private startStream(client: QueryClient, params: QueryParamsUI, gen: number) {
    const guard =
      <A extends unknown[]>(fn: (...args: A) => void) =>
      (...args: A) => {
        if (gen !== this.generation) return
        fn(...args)
      }
    const closer = client.streamQueryUI(params, {
      onPartial: guard((flows, summary) => {
        const patch: Partial<RunnerSnapshot> = {}
        // server may emit partials with no row data yet; only replace when rows exist
        if (Array.isArray(flows) && flows.length > 0) {
          patch.rows = flows
          patch.partial = true
        }
        if (summary) patch.summary = summary
        if (Object.keys(patch).length > 0) this.transition(patch)
      }),
      onFinal: guard((flows, summary) => {
        this.streamCloser = null
        // always take the final, server-sorted result
        this.transition({
          phase: 'done',
          partial: false,
          rows: flows,
          summary: summary ?? this.snapshot.summary,
        })
      }),
      onProgress: guard((p) => this.transition({ progress: p || {} })),
      onMeta: guard((meta) => {
        const patch: Partial<RunnerSnapshot> = {}
        if (meta?.hostsStatuses) patch.hostsStatuses = meta.hostsStatuses
        if (typeof meta?.hostErrorCount === 'number') patch.hostErrorCount = meta.hostErrorCount
        if (typeof meta?.hostOkCount === 'number') patch.hostOkCount = meta.hostOkCount
        if (Object.keys(patch).length > 0) this.transition(patch)
      }),
      onError: guard((err) => {
        if (isApiError(err)) {
          // connection-level failure: fatal to the Run
          if (this.streamCloser) {
            try {
              this.streamCloser.close()
            } catch {}
            this.streamCloser = null
          }
          this.transition({ phase: 'error', partial: false, runError: err })
          return
        }
        // per-host failure: accumulate as Host Error, the Run continues
        const host = typeof err?.host === 'string' ? err.host : undefined
        const message = typeof err?.message === 'string' ? err.message : 'stream error'
        this.transition({ hostErrors: [...this.snapshot.hostErrors, { host, message }] })
      }),
    })
    // events may have fired synchronously during creation (supersession or a
    // fatal error); only keep the closer while this Run is still the live one
    if (gen === this.generation && this.snapshot.phase === 'running') {
      this.streamCloser = closer
    } else {
      try {
        closer.close()
      } catch {}
    }
  }
}
