import { describe, it, expect } from 'vitest'
import { interpretSSEEvent, unwrapPayload, countHostStatuses } from './sseEvents'

const row = (sip: string, dip: string) => ({
  attributes: { sip, dip, dport: 443, proto: 6 },
  counters: { br: 1, bs: 1, pr: 1, ps: 1 },
  labels: {},
})

describe('unwrapPayload', () => {
  it('wraps a bare array into { rows }', () => {
    expect(unwrapPayload([1, 2])).toEqual({ rows: [1, 2] })
  })

  it('unwraps nested result/partialResult/finalResult envelopes', () => {
    expect(unwrapPayload({ result: { rows: [1] } })).toEqual({ rows: [1] })
    expect(unwrapPayload({ partialResult: { rows: [1] } })).toEqual({ rows: [1] })
    expect(unwrapPayload({ finalResult: { rows: [1] } })).toEqual({ rows: [1] })
  })

  it('stops after bounded iterations on self-referential wrappers', () => {
    const loop: any = {}
    loop.result = loop
    // must terminate, not hang
    expect(unwrapPayload(loop)).toBe(loop)
  })

  it('normalizes data/flows arrays to rows', () => {
    expect(unwrapPayload({ data: [1] }).rows).toEqual([1])
    expect(unwrapPayload({ flows: [1] }).rows).toEqual([1])
  })
})

describe('countHostStatuses', () => {
  it('returns undefined when no status map is present', () => {
    expect(countHostStatuses(undefined)).toBeUndefined()
    expect(countHostStatuses('nope')).toBeUndefined()
  })

  it('tallies ok vs error (non-ok counts as error)', () => {
    const c = countHostStatuses({
      h1: { code: 'ok' },
      h2: { code: 'OK' },
      h3: { code: 'timeout' },
      h4: {},
    })
    expect(c).toMatchObject({ hostOkCount: 2, hostErrorCount: 2 })
  })
})

describe('interpretSSEEvent', () => {
  it('maps a partialResult event to a partial outcome with flows', () => {
    const o = interpretSSEEvent({
      event: 'partialResult',
      data: JSON.stringify({ rows: [row('1.1.1.1', '2.2.2.2')] }),
    })
    expect(o.kind).toBe('partial')
    if (o.kind === 'partial') expect(o.flows).toHaveLength(1)
  })

  it('maps a finalResult event to a final outcome', () => {
    const o = interpretSSEEvent({
      event: 'finalResult',
      data: JSON.stringify({ rows: [row('1.1.1.1', '2.2.2.2')] }),
    })
    expect(o.kind).toBe('final')
  })

  it('carries host-status meta when present', () => {
    const o = interpretSSEEvent({
      event: 'partialResult',
      data: JSON.stringify({ rows: [], hosts_statuses: { h1: { code: 'ok' }, h2: { code: 'err' } } }),
    })
    expect(o.kind).toBe('partial')
    if (o.kind === 'partial') expect(o.meta).toMatchObject({ hostOkCount: 1, hostErrorCount: 1 })
  })

  it('treats an error event as a non-fatal Host Error outcome (not final)', () => {
    const o = interpretSSEEvent({ event: 'error', data: JSON.stringify({ message: 'host down', host: 'h1' }) })
    expect(o.kind).toBe('error')
    if (o.kind === 'error') expect((o.error as any).message).toBe('host down')
  })

  it('synthesizes an error for an empty error event', () => {
    const o = interpretSSEEvent({ event: 'error' })
    expect(o.kind).toBe('error')
  })

  it('maps a progress event', () => {
    const o = interpretSSEEvent({ event: 'progress', data: JSON.stringify({ done: 3, total: 10 }) })
    expect(o).toEqual({ kind: 'progress', progress: { done: 3, total: 10 } })
  })

  it('ignores a message event with no data', () => {
    expect(interpretSSEEvent({ event: 'message' })).toEqual({ kind: 'ignore' })
  })

  it('treats an unnamed (message) event with data as partial', () => {
    const o = interpretSSEEvent({ data: JSON.stringify({ rows: [row('a', 'b')] }) })
    expect(o.kind).toBe('partial')
  })

  it('falls back to partial/final by detecting rows on named-but-unrecognized events', () => {
    const partial = interpretSSEEvent({ event: 'chunk', data: JSON.stringify({ rows: [row('a', 'b')] }) })
    expect(partial.kind).toBe('partial')
    const final = interpretSSEEvent({
      event: 'chunk',
      data: JSON.stringify({ rows: [row('a', 'b')], final: true }),
    })
    expect(final.kind).toBe('final')
  })

  it('ignores unrecognized events without rows', () => {
    expect(interpretSSEEvent({ event: 'whatever', data: JSON.stringify({ noise: 1 }) })).toEqual({
      kind: 'ignore',
    })
  })

  it('yields an error outcome when partial payload extraction throws', () => {
    // rows is a string, so extractFlows calls .map on a non-array and throws;
    // the partial branch must convert that into a non-fatal error outcome.
    const o = interpretSSEEvent({ event: 'partial', data: '{"rows": "not-an-array"}' })
    expect(o.kind).toBe('error')
  })
})
