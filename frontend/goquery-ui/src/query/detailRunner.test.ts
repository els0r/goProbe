import { describe, it, expect } from 'vitest'
import {
  DetailRunner,
  DetailQueryClient,
  DetailBase,
  deriveDetailQuery,
  deriveDrillQuery,
  rowMeta,
} from './detailRunner'
import { QueryRunner } from './runner'
import { FlowRecord } from '../flows'
import { SummarySchema } from '../api/domain'
import { QueryParamsUI } from './params'
import { ApiError } from '../api/errors'

const PARAMS: QueryParamsUI = {
  first: '-1h',
  last: '',
  ifaces: 'eth0',
  query: 'sip,dip',
  condition: 'proto eq tcp',
  limit: 100,
  sort_by: 'bytes',
  sort_ascending: false,
}

function flow(over?: Partial<FlowRecord>): FlowRecord {
  const r = {
    sip: '10.0.0.1',
    dip: '10.0.0.2',
    dport: 443,
    proto: 6,
    bytes_in: 1,
    bytes_out: 1,
    packets_in: 1,
    packets_out: 1,
    host: 'web-1',
    host_id: 'h1',
    iface: 'eth0',
    bidirectional: true,
    _raw: { attributes: {}, counters: {} },
    ...over,
  }
  // derive totals from the merged counters, mirroring flattenRow
  return { ...r, bytes_total: r.bytes_in + r.bytes_out, packets_total: r.packets_in + r.packets_out }
}

function base(over?: Partial<DetailBase>): DetailBase {
  return { params: PARAMS, rows: [flow()], ...over }
}

function apiError(over?: Partial<ApiError>): ApiError {
  return { name: 'ApiError', message: 'boom', category: 'client', status: 400, ...over }
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

type RunResult = { flows: FlowRecord[]; summary?: SummarySchema }

class FakeClient implements DetailQueryClient {
  requests: Array<{ params: QueryParamsUI; signal?: AbortSignal; d: Deferred<RunResult> }> = []

  runQueryUI(params: QueryParamsUI, signal?: AbortSignal): Promise<RunResult> {
    const d = deferred<RunResult>()
    this.requests.push({ params, signal, d })
    return d.promise
  }
}

function makeRunner() {
  const client = new FakeClient()
  const runner = new DetailRunner(() => client)
  return { client, runner }
}

// lets awaited continuations settle
const tick = () => new Promise<void>((r) => setTimeout(r, 0))

describe('query recipes', () => {
  it('ip: proto,dport over hosts carrying the IP, base condition preserved', () => {
    const rows = [
      flow({ sip: '1.2.3.4', host_id: 'h1' }),
      flow({ dip: '1.2.3.4', host_id: 'h2' }),
      flow({ host_id: 'h3' }), // unrelated host must not be scoped in
    ]
    const q = deriveDetailQuery({ kind: 'ip', ip: '1.2.3.4' }, base({ rows }))
    expect(q.query).toBe('proto,dport')
    expect(q.condition).toBe('(proto eq tcp) and (host=1.2.3.4)')
    expect(q.query_hosts).toBe('h1,h2')
    expect(q.hosts_resolver).toBe('string')
    expect(q.sort_by).toBe('bytes')
    expect(q.sort_ascending).toBe(false)
  })

  it('ip: without base condition uses the bare host condition', () => {
    const q = deriveDetailQuery(
      { kind: 'ip', ip: '1.2.3.4' },
      base({ params: { ...PARAMS, condition: undefined }, rows: [] })
    )
    expect(q.condition).toBe('host=1.2.3.4')
    expect(q.query_hosts).toBeUndefined()
  })

  it('iface: scopes to host and interface and clears the condition', () => {
    const q = deriveDetailQuery({ kind: 'iface', hostId: 'h1', iface: 'eth1' }, base())
    expect(q.query).toBe('iface,port,protocol')
    expect(q.query_hosts).toBe('h1')
    expect(q.ifaces).toBe('eth1')
    expect(q.condition).toBeUndefined()
    expect(q.hosts_resolver).toBe('string')
  })

  it('host: queries interfaces scoped to the host', () => {
    const q = deriveDetailQuery({ kind: 'host', hostId: 'h1' }, base())
    expect(q.query).toBe('iface')
    expect(q.query_hosts).toBe('h1')
    expect(q.condition).toBeUndefined()
  })

  it('temporal: time series conditioned on the row 5-tuple plus base condition', () => {
    const q = deriveDetailQuery({ kind: 'temporal', row: flow() }, base())
    expect(q.query).toBe('time')
    expect(q.condition).toBe('sip=10.0.0.1 and dip=10.0.0.2 and dport=443 and proto=6 and proto eq tcp')
    expect(q.query_hosts).toBe('h1')
    expect(q.ifaces).toBe('eth0')
    expect(q.limit).toBe(100000)
  })

  it('temporal: null tuple fields are omitted from the condition', () => {
    const q = deriveDetailQuery(
      { kind: 'temporal', row: flow({ dport: null, proto: null }) },
      base({ params: { ...PARAMS, condition: undefined } })
    )
    expect(q.condition).toBe('sip=10.0.0.1 and dip=10.0.0.2')
  })

  it('drill: bucket window, complement attributes, tuple + base condition', () => {
    const meta = rowMeta(flow())
    const q = deriveDrillQuery(
      { startMs: Date.UTC(2026, 0, 1, 10), endMs: Date.UTC(2026, 0, 1, 11) },
      { values: ['sip', 'dip'], all: false },
      meta,
      PARAMS
    )
    expect(q.first).toBe('2026-01-01T10:00:00.000Z')
    expect(q.last).toBe('2026-01-01T11:00:00.000Z')
    expect(q.query).toBe('sip,dip')
    expect(q.query_hosts).toBe('h1')
    expect(q.ifaces).toBe('eth0')
    expect(q.condition).toBe('sip=10.0.0.1 and dip=10.0.0.2 and dport=443 and proto=6 and proto eq tcp')
    expect(q.limit).toBe(1000)
    expect(q.hosts_resolver).toBe('string')
  })
})

describe('panel lifecycle', () => {
  it('open walks loading → done and lands rows and summary', async () => {
    const { client, runner } = makeRunner()
    runner.open({ kind: 'ip', ip: '1.2.3.4' }, base())
    expect(runner.getSnapshot().panel?.phase).toBe('loading')

    client.requests[0].d.resolve({
      flows: [flow()],
      summary: { hits: { total: 1 } } as unknown as SummarySchema,
    })
    await tick()
    const panel = runner.getSnapshot().panel!
    expect(panel.phase).toBe('done')
    expect(panel.rows).toHaveLength(1)
    expect(panel.summary).toBeDefined()
    expect(panel.error).toBeNull()
  })

  it('a failed Detail Run lands in error with the raw ApiError', async () => {
    const { client, runner } = makeRunner()
    runner.open({ kind: 'host', hostId: 'h1' }, base())
    client.requests[0].d.reject(apiError())
    await tick()
    const panel = runner.getSnapshot().panel!
    expect(panel.phase).toBe('error')
    expect(panel.error?.category).toBe('client')
  })

  it('resolves the host name from the base rows', () => {
    const { runner } = makeRunner()
    runner.open({ kind: 'host', hostId: 'h1' }, base())
    const panel = runner.getSnapshot().panel!
    expect(panel.kind === 'host' && panel.hostName).toBe('web-1')
  })

  it('temporal panel carries row meta and the attributes shown by the Run', () => {
    const { runner } = makeRunner()
    runner.open({ kind: 'temporal', row: flow() }, base())
    const panel = runner.getSnapshot().panel!
    if (panel.kind !== 'temporal') throw new Error('expected temporal panel')
    expect(panel.meta.host).toBe('web-1')
    expect(panel.attrsShown).toEqual(['sip', 'dip'])
  })

  it('targets with empty identifiers are ignored', () => {
    const { client, runner } = makeRunner()
    runner.open({ kind: 'ip', ip: '' }, base())
    runner.open({ kind: 'iface', hostId: '', iface: 'eth0' }, base())
    runner.open({ kind: 'iface', hostId: 'h1', iface: '' }, base())
    runner.open({ kind: 'host', hostId: '' }, base())
    expect(runner.getSnapshot().panel).toBeNull()
    expect(client.requests).toHaveLength(0)
  })

  it('opening the already-open target closes it (toggle)', async () => {
    const { client, runner } = makeRunner()
    const row = flow()
    runner.open({ kind: 'temporal', row }, base())
    client.requests[0].d.resolve({ flows: [flow()] })
    await tick()
    expect(runner.getSnapshot().panel).not.toBeNull()

    runner.open({ kind: 'temporal', row: { ...row } }, base())
    expect(runner.getSnapshot().panel).toBeNull()
    expect(client.requests).toHaveLength(1)
  })

  it('a new Detail Run supersedes the in-flight one; its late response is inert', async () => {
    const { client, runner } = makeRunner()
    runner.open({ kind: 'ip', ip: '1.2.3.4' }, base())
    runner.open({ kind: 'ip', ip: '5.6.7.8' }, base())
    expect(client.requests[0].signal?.aborted).toBe(true)

    client.requests[0].d.resolve({ flows: [flow(), flow()] })
    await tick()
    const panel = runner.getSnapshot().panel!
    expect(panel.kind === 'ip' && panel.ip).toBe('5.6.7.8')
    expect(panel.phase).toBe('loading')

    client.requests[1].d.resolve({ flows: [flow()] })
    await tick()
    expect(runner.getSnapshot().panel?.phase).toBe('done')
    expect(runner.getSnapshot().panel?.rows).toHaveLength(1)
  })

  it('close aborts the in-flight Detail Run and drops its late events', async () => {
    const { client, runner } = makeRunner()
    runner.open({ kind: 'ip', ip: '1.2.3.4' }, base())
    runner.close()
    expect(runner.getSnapshot().panel).toBeNull()
    expect(client.requests[0].signal?.aborted).toBe(true)

    client.requests[0].d.resolve({ flows: [flow()] })
    await tick()
    expect(runner.getSnapshot().panel).toBeNull()
  })
})

describe('drill-down lifecycle', () => {
  async function openTemporal() {
    const { client, runner } = makeRunner()
    runner.open({ kind: 'temporal', row: flow() }, base())
    client.requests[0].d.resolve({ flows: [flow()] })
    await tick()
    return { client, runner }
  }
  const BUCKET = { startMs: 1000, endMs: 2000 }
  const ATTRS = { values: ['dport', 'proto'], all: false }

  it('openDrill walks loading → done within the open temporal panel', async () => {
    const { client, runner } = await openTemporal()
    runner.openDrill(BUCKET, ATTRS)
    expect(runner.getSnapshot().drill?.phase).toBe('loading')
    expect(runner.getSnapshot().drill?.attrs).toEqual(['dport', 'proto'])

    client.requests[1].d.resolve({ flows: [flow()] })
    await tick()
    expect(runner.getSnapshot().drill?.phase).toBe('done')
    expect(runner.getSnapshot().drill?.rows).toHaveLength(1)
    // the panel is untouched
    expect(runner.getSnapshot().panel?.phase).toBe('done')
  })

  it('no-ops without an open temporal panel or without attributes', () => {
    const { client, runner } = makeRunner()
    runner.openDrill(BUCKET, ATTRS)
    expect(client.requests).toHaveLength(0)

    runner.open({ kind: 'ip', ip: '1.2.3.4' }, base())
    runner.openDrill(BUCKET, ATTRS)
    expect(client.requests).toHaveLength(1) // only the panel fetch
  })

  it('a new Drill-down supersedes the in-flight one', async () => {
    const { client, runner } = await openTemporal()
    runner.openDrill(BUCKET, ATTRS)
    runner.openDrill({ startMs: 3000, endMs: 4000 }, ATTRS)
    expect(client.requests[1].signal?.aborted).toBe(true)

    client.requests[1].d.resolve({ flows: [flow(), flow()] })
    await tick()
    expect(runner.getSnapshot().drill?.phase).toBe('loading')
    expect(runner.getSnapshot().drill?.bucket.startMs).toBe(3000)
  })

  it('closeDrill discards the Drill-down but keeps the panel', async () => {
    const { client, runner } = await openTemporal()
    runner.openDrill(BUCKET, ATTRS)
    runner.closeDrill()
    expect(client.requests[1].signal?.aborted).toBe(true)
    expect(runner.getSnapshot().drill).toBeNull()
    expect(runner.getSnapshot().panel).not.toBeNull()
  })

  it('supersession cascades: opening a new Detail Run discards the Drill-down', async () => {
    const { client, runner } = await openTemporal()
    runner.openDrill(BUCKET, ATTRS)
    runner.open({ kind: 'ip', ip: '9.9.9.9' }, base())
    expect(client.requests[1].signal?.aborted).toBe(true)
    expect(runner.getSnapshot().drill).toBeNull()

    // the superseded drill's late response is inert
    client.requests[1].d.resolve({ flows: [flow()] })
    await tick()
    expect(runner.getSnapshot().drill).toBeNull()
  })
})

describe('Run observation', () => {
  it('a new Run closes the panel and aborts the Detail Run', () => {
    const detailClient = new FakeClient()
    const runClient = {
      validateQueryUI: () => new Promise<void>(() => {}),
      runQueryUI: () => new Promise<never>(() => {}),
      streamQueryUI: () => ({ close: () => {} }),
    }
    const observed = new QueryRunner(() => runClient)
    const detail = new DetailRunner(() => detailClient, observed)

    detail.open({ kind: 'ip', ip: '1.2.3.4' }, base())
    expect(detail.getSnapshot().panel).not.toBeNull()

    // a real Run start: phase enters 'validating'
    observed.run(PARAMS, { stream: false })
    expect(detail.getSnapshot().panel).toBeNull()
    expect(detailClient.requests[0].signal?.aborted).toBe(true)

    detail.dispose()
  })

  it('standalone Validation and cancel do not close the panel', async () => {
    const detailClient = new FakeClient()
    const validations: Array<Deferred<void>> = []
    const runClient = {
      validateQueryUI: () => {
        const d = deferred<void>()
        validations.push(d)
        return d.promise
      },
      runQueryUI: () => new Promise<never>(() => {}),
      streamQueryUI: () => ({ close: () => {} }),
    }
    const observed = new QueryRunner(() => runClient)
    const detail = new DetailRunner(() => detailClient, observed)

    detail.open({ kind: 'ip', ip: '1.2.3.4' }, base())
    void observed.validate(PARAMS)
    validations[0].resolve()
    await tick()
    expect(detail.getSnapshot().panel).not.toBeNull()

    observed.cancel()
    expect(detail.getSnapshot().panel).not.toBeNull()
    detail.dispose()
  })
})

describe('snapshot contract', () => {
  it('is reference-stable between transitions and replaced on each transition', async () => {
    const { client, runner } = makeRunner()
    const before = runner.getSnapshot()
    expect(runner.getSnapshot()).toBe(before)

    runner.open({ kind: 'ip', ip: '1.2.3.4' }, base())
    const during = runner.getSnapshot()
    expect(during).not.toBe(before)
    expect(runner.getSnapshot()).toBe(during)

    client.requests[0].d.resolve({ flows: [] })
    await tick()
    expect(runner.getSnapshot()).not.toBe(during)
  })

  it('notifies subscribers on every transition and stops after unsubscribe', async () => {
    const { runner } = makeRunner()
    let calls = 0
    const unsubscribe = runner.subscribe(() => calls++)

    runner.open({ kind: 'ip', ip: '1.2.3.4' }, base())
    expect(calls).toBe(1)
    runner.close()
    expect(calls).toBe(2)

    unsubscribe()
    runner.open({ kind: 'ip', ip: '5.6.7.8' }, base())
    expect(calls).toBe(2)
  })
})
