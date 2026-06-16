import { describe, it, expect } from 'vitest'
import { QueryRunner, QueryClient, StreamHandlers, HostsStatuses } from './runner'
import { FlowRecord } from '../flows'
import { SummarySchema } from '../api/domain'
import { QueryParamsUI } from './params'
import { ApiError } from '../api/errors'

const PARAMS: QueryParamsUI = {
  first: '-1h',
  last: '',
  ifaces: 'eth0',
  query: 'sip,dip',
  limit: 100,
  sort_by: 'bytes',
  sort_ascending: false,
}

function flow(sip: string, dip: string): FlowRecord {
  return {
    sip,
    dip,
    dport: 443,
    proto: 6,
    bytes_in: 1,
    bytes_out: 1,
    bytes_total: 2,
    packets_in: 1,
    packets_out: 1,
    packets_total: 2,
    bidirectional: true,
    _raw: { attributes: {}, counters: {} },
  }
}

function apiError(over?: Partial<ApiError>): ApiError {
  return { name: 'ApiError', message: 'boom', category: 'client', status: 400, ...over }
}

function abortError(): ApiError {
  return { name: 'AbortError', message: 'request aborted', category: 'network' }
}

interface Deferred<T> {
  promise: Promise<T>
  resolve: (v: T) => void
  reject: (e: unknown) => void
}

function deferred<T>(): Deferred<T> {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

type RunResult = { flows: FlowRecord[]; summary?: SummarySchema; hostsStatuses?: HostsStatuses }

class FakeClient implements QueryClient {
  validations: Array<{ params: QueryParamsUI; signal?: AbortSignal; d: Deferred<void> }> = []
  requests: Array<{ params: QueryParamsUI; signal?: AbortSignal; d: Deferred<RunResult> }> = []
  streams: Array<{ params: QueryParamsUI; handlers: StreamHandlers; closed: boolean }> = []

  validateQueryUI(params: QueryParamsUI, signal?: AbortSignal): Promise<void> {
    const d = deferred<void>()
    this.validations.push({ params, signal, d })
    return d.promise
  }

  runQueryUI(params: QueryParamsUI, signal?: AbortSignal): Promise<RunResult> {
    const d = deferred<RunResult>()
    this.requests.push({ params, signal, d })
    return d.promise
  }

  streamQueryUI(params: QueryParamsUI, handlers: StreamHandlers): { close: () => void } {
    const s = { params, handlers, closed: false }
    this.streams.push(s)
    return {
      close: () => {
        s.closed = true
      },
    }
  }
}

function makeRunner() {
  const client = new FakeClient()
  const runner = new QueryRunner(() => client)
  return { client, runner }
}

// lets the runner's awaited continuations settle
const tick = () => new Promise<void>((r) => setTimeout(r, 0))

describe('non-streaming run', () => {
  it('walks idle → validating → running → done and lands rows, summary, host counts', async () => {
    const { client, runner } = makeRunner()
    const phases: string[] = []
    runner.subscribe(() => phases.push(runner.getSnapshot().phase))

    expect(runner.getSnapshot().phase).toBe('idle')
    runner.run(PARAMS, { stream: false })
    expect(runner.getSnapshot().phase).toBe('validating')

    client.validations[0].d.resolve()
    await tick()
    expect(runner.getSnapshot().phase).toBe('running')
    expect(client.requests).toHaveLength(1)

    client.requests[0].d.resolve({
      flows: [flow('10.0.0.1', '10.0.0.2')],
      summary: { hits: { total: 1 } } as unknown as SummarySchema,
      hostsStatuses: { a: { code: 'ok' }, b: { code: 'error', message: 'nope' } },
    })
    await tick()

    const snap = runner.getSnapshot()
    expect(snap.phase).toBe('done')
    expect(snap.rows).toHaveLength(1)
    expect(snap.partial).toBe(false)
    expect(snap.hostOkCount).toBe(1)
    expect(snap.hostErrorCount).toBe(1)
    expect(snap.runError).toBeNull()
    expect(phases).toEqual(['validating', 'running', 'done'])
  })

  it('clears previous rows at run start', async () => {
    const { client, runner } = makeRunner()
    runner.run(PARAMS, { stream: false })
    client.validations[0].d.resolve()
    await tick()
    client.requests[0].d.resolve({ flows: [flow('1.1.1.1', '2.2.2.2')] })
    await tick()
    expect(runner.getSnapshot().rows).toHaveLength(1)

    runner.run(PARAMS, { stream: false })
    expect(runner.getSnapshot().rows).toHaveLength(0)
    expect(runner.getSnapshot().summary).toBeUndefined()
  })

  it('failed run lands in error with runError set', async () => {
    const { client, runner } = makeRunner()
    runner.run(PARAMS, { stream: false })
    client.validations[0].d.resolve()
    await tick()
    client.requests[0].d.reject(apiError())
    await tick()
    const snap = runner.getSnapshot()
    expect(snap.phase).toBe('error')
    expect(snap.runError?.category).toBe('client')
    expect(snap.validationError).toBeNull()
  })
})

describe('preflight validation', () => {
  it('on failure returns to idle with validationError and never executes the query', async () => {
    const { client, runner } = makeRunner()
    runner.run(PARAMS, { stream: false })
    client.validations[0].d.reject(apiError({ status: 422 }))
    await tick()
    const snap = runner.getSnapshot()
    expect(snap.phase).toBe('idle')
    expect(snap.validationError?.status).toBe(422)
    expect(client.requests).toHaveLength(0)
    expect(client.streams).toHaveLength(0)
  })

  it('success clears a previous validationError', async () => {
    const { client, runner } = makeRunner()
    void runner.validate(PARAMS)
    client.validations[0].d.reject(apiError({ status: 422 }))
    await tick()
    expect(runner.getSnapshot().validationError).not.toBeNull()

    runner.run(PARAMS, { stream: false })
    client.validations[1].d.resolve()
    await tick()
    expect(runner.getSnapshot().validationError).toBeNull()
  })
})

describe('standalone validate', () => {
  it('returns verdict and records validationError without touching phase', async () => {
    const { client, runner } = makeRunner()
    const p = runner.validate(PARAMS)
    client.validations[0].d.reject(apiError({ status: 422 }))
    expect(await p).toBe(false)
    expect(runner.getSnapshot().phase).toBe('idle')
    expect(runner.getSnapshot().validationError?.status).toBe(422)

    const p2 = runner.validate(PARAMS)
    client.validations[1].d.resolve()
    expect(await p2).toBe(true)
    expect(runner.getSnapshot().validationError).toBeNull()
  })

  it('a newer validate supersedes an older in-flight one', async () => {
    const { client, runner } = makeRunner()
    const first = runner.validate(PARAMS)
    const second = runner.validate(PARAMS)
    expect(client.validations[0].signal?.aborted).toBe(true)

    client.validations[0].d.reject(abortError())
    client.validations[1].d.reject(apiError({ status: 422 }))
    expect(await first).toBe(false)
    expect(await second).toBe(false)
    // only the second verdict landed
    expect(runner.getSnapshot().validationError?.status).toBe(422)
  })
})

describe('supersession', () => {
  it('a new run makes the previous run inert, even when its response arrives later', async () => {
    const { client, runner } = makeRunner()
    runner.run(PARAMS, { stream: false })
    client.validations[0].d.resolve()
    await tick()

    runner.run(PARAMS, { stream: false })
    expect(client.requests[0].signal?.aborted).toBe(true)
    client.validations[1].d.resolve()
    await tick()

    // late events from the superseded run must not land
    client.requests[0].d.resolve({ flows: [flow('9.9.9.9', '8.8.8.8')] })
    await tick()
    expect(runner.getSnapshot().rows).toHaveLength(0)
    expect(runner.getSnapshot().phase).toBe('running')

    client.requests[1].d.resolve({ flows: [flow('1.1.1.1', '2.2.2.2')] })
    await tick()
    expect(runner.getSnapshot().phase).toBe('done')
    expect(runner.getSnapshot().rows[0].sip).toBe('1.1.1.1')
  })

  it('a new run closes the previous stream', async () => {
    const { client, runner } = makeRunner()
    runner.run(PARAMS, { stream: true })
    client.validations[0].d.resolve()
    await tick()
    expect(client.streams).toHaveLength(1)

    runner.run(PARAMS, { stream: true })
    expect(client.streams[0].closed).toBe(true)

    // late final from the closed stream is inert
    client.streams[0].handlers.onFinal?.([flow('9.9.9.9', '8.8.8.8')], undefined)
    expect(runner.getSnapshot().rows).toHaveLength(0)
  })
})

describe('cancel', () => {
  it('aborts in-flight work, transitions to idle synchronously, and drops late events', async () => {
    const { client, runner } = makeRunner()
    runner.run(PARAMS, { stream: false })
    client.validations[0].d.resolve()
    await tick()

    runner.cancel()
    expect(runner.getSnapshot().phase).toBe('idle')
    expect(client.requests[0].signal?.aborted).toBe(true)

    client.requests[0].d.reject(abortError())
    await tick()
    expect(runner.getSnapshot().phase).toBe('idle')
    expect(runner.getSnapshot().runError).toBeNull()
  })

  it('clears transient state and errors but keeps phase consistent', async () => {
    const { client, runner } = makeRunner()
    runner.run(PARAMS, { stream: false })
    client.validations[0].d.reject(apiError())
    await tick()
    expect(runner.getSnapshot().validationError).not.toBeNull()

    runner.cancel()
    const snap = runner.getSnapshot()
    expect(snap.validationError).toBeNull()
    expect(snap.runError).toBeNull()
    expect(snap.hostErrors).toEqual([])
    expect(snap.progress).toEqual({})
  })
})

describe('streaming run', () => {
  async function startStreaming() {
    const { client, runner } = makeRunner()
    runner.run(PARAMS, { stream: true })
    client.validations[0].d.resolve()
    await tick()
    return { client, runner, handlers: client.streams[0].handlers }
  }

  it('partials mark rows partial; final settles done with server-sorted rows', async () => {
    const { runner, handlers } = await startStreaming()
    expect(runner.getSnapshot().phase).toBe('running')

    handlers.onPartial?.([flow('1.1.1.1', '2.2.2.2')], undefined)
    expect(runner.getSnapshot().partial).toBe(true)
    expect(runner.getSnapshot().rows).toHaveLength(1)

    // row-less partial must not wipe rows already shown
    handlers.onPartial?.([], { hits: { total: 5 } } as unknown as SummarySchema)
    expect(runner.getSnapshot().rows).toHaveLength(1)
    expect(runner.getSnapshot().summary).toBeDefined()

    handlers.onFinal?.([flow('3.3.3.3', '4.4.4.4')], undefined)
    const snap = runner.getSnapshot()
    expect(snap.phase).toBe('done')
    expect(snap.partial).toBe(false)
    expect(snap.rows[0].sip).toBe('3.3.3.3')
    // final without summary keeps the last streamed summary
    expect(snap.summary).toBeDefined()
  })

  it('per-host errors accumulate as hostErrors and the run continues', async () => {
    const { runner, handlers } = await startStreaming()
    handlers.onError?.({ message: 'host down', host: 'h1' })
    handlers.onError?.({ message: 'timeout' })
    const snap = runner.getSnapshot()
    expect(snap.phase).toBe('running')
    expect(snap.hostErrors).toEqual([
      { host: 'h1', message: 'host down' },
      { host: undefined, message: 'timeout' },
    ])
  })

  it('a connection-level ApiError is fatal: phase error, stream closed', async () => {
    const { client, runner, handlers } = await startStreaming()
    handlers.onError?.(apiError({ category: 'network', status: undefined }))
    const snap = runner.getSnapshot()
    expect(snap.phase).toBe('error')
    expect(snap.runError?.category).toBe('network')
    expect(client.streams[0].closed).toBe(true)
  })

  it('progress and meta events update the snapshot', async () => {
    const { runner, handlers } = await startStreaming()
    handlers.onProgress?.({ done: 3, total: 10 })
    handlers.onMeta?.({ hostsStatuses: { a: { code: 'ok' } }, hostOkCount: 1, hostErrorCount: 0 })
    const snap = runner.getSnapshot()
    expect(snap.progress).toEqual({ done: 3, total: 10 })
    expect(snap.hostsStatuses).toEqual({ a: { code: 'ok' } })
    expect(snap.hostOkCount).toBe(1)
  })
})

describe('snapshot contract', () => {
  it('is reference-stable between transitions and replaced on each transition', async () => {
    const { client, runner } = makeRunner()
    const before = runner.getSnapshot()
    expect(runner.getSnapshot()).toBe(before)

    runner.run(PARAMS, { stream: false })
    const during = runner.getSnapshot()
    expect(during).not.toBe(before)
    expect(runner.getSnapshot()).toBe(during)

    client.validations[0].d.resolve()
    await tick()
    expect(runner.getSnapshot()).not.toBe(during)
  })

  it('notifies subscribers on every transition and stops after unsubscribe', async () => {
    const { client, runner } = makeRunner()
    let calls = 0
    const unsubscribe = runner.subscribe(() => calls++)

    runner.run(PARAMS, { stream: false })
    expect(calls).toBe(1)
    client.validations[0].d.resolve()
    await tick()
    expect(calls).toBe(2)

    unsubscribe()
    runner.cancel()
    expect(calls).toBe(2)
  })
})
