import { describe, it, expect } from 'vitest'
import { structuralKey, isDirty, runButtonState } from './autoRun'
import { QueryParamsUI } from './params'

const base: QueryParamsUI = {
  first: '-24h',
  last: 'now',
  ifaces: 'eth0',
  query: 'sip,dip',
  query_hosts: 'host1',
  hosts_resolver: 'default',
  condition: 'dport=443',
  limit: 100,
  sort_by: 'bytes',
  sort_ascending: false,
  in_only: false,
  out_only: false,
  sum: false,
}

describe('structuralKey', () => {
  it('is stable for the same params', () => {
    expect(structuralKey(base)).toBe(structuralKey({ ...base }))
  })

  it('changes when a structured field changes', () => {
    const changes: Partial<QueryParamsUI>[] = [
      { first: '-48h' },
      { last: '-1h' },
      { query: 'sip,dip,dport' },
      { sort_by: 'packets' },
      { sort_ascending: true },
      { limit: 50 },
    ]
    for (const change of changes) {
      expect(structuralKey({ ...base, ...change })).not.toBe(structuralKey(base))
    }
  })

  it('is unchanged when a free-text or non-structured field changes', () => {
    const changes: Partial<QueryParamsUI>[] = [
      { query_hosts: 'host2' },
      { ifaces: 'eth1' },
      { condition: 'dport=80' },
      { hosts_resolver: 'reverse' },
      { in_only: true },
      { out_only: true },
      { sum: true },
    ]
    for (const change of changes) {
      expect(structuralKey({ ...base, ...change })).toBe(structuralKey(base))
    }
  })
})

describe('isDirty', () => {
  it('returns false when lastRun is null', () => {
    expect(isDirty(base, null)).toBe(false)
  })

  it('returns false when all three free-text fields match', () => {
    expect(isDirty({ ...base }, base)).toBe(false)
  })

  it('returns true when query_hosts differs', () => {
    expect(isDirty({ ...base, query_hosts: 'host2' }, base)).toBe(true)
  })

  it('returns true when ifaces differs', () => {
    expect(isDirty({ ...base, ifaces: 'eth1' }, base)).toBe(true)
  })

  it('returns true when condition differs', () => {
    expect(isDirty({ ...base, condition: 'dport=80' }, base)).toBe(true)
  })

  it("treats '' and undefined as equal (not dirty)", () => {
    expect(isDirty({ ...base, query_hosts: '' }, { ...base, query_hosts: undefined })).toBe(false)
  })

  it("treats whitespace as empty (not dirty)", () => {
    expect(isDirty({ ...base, condition: '  ' }, { ...base, condition: '' })).toBe(false)
  })

  it('trims before comparing (not dirty)', () => {
    expect(isDirty({ ...base, ifaces: ' x ' }, { ...base, ifaces: 'x' })).toBe(false)
  })

  it('is unchanged when a non-free-text field changes', () => {
    const changes: Partial<QueryParamsUI>[] = [
      { limit: 50 },
      { sort_by: 'packets' },
      { first: '-48h' },
    ]
    for (const change of changes) {
      expect(isDirty({ ...base, ...change }, base)).toBe(false)
    }
  })
})

describe('runButtonState', () => {
  const T = 450

  it('idle + clean ⇒ refresh', () => {
    expect(runButtonState({ phase: 'idle', dirty: false, elapsedMs: 0, threshold: T })).toBe(
      'refresh'
    )
  })

  it('idle + dirty ⇒ apply', () => {
    expect(runButtonState({ phase: 'idle', dirty: true, elapsedMs: 0, threshold: T })).toBe('apply')
  })

  it("'done' behaves like idle (clean ⇒ refresh, dirty ⇒ apply)", () => {
    expect(runButtonState({ phase: 'done', dirty: false, elapsedMs: 0, threshold: T })).toBe(
      'refresh'
    )
    expect(runButtonState({ phase: 'done', dirty: true, elapsedMs: 0, threshold: T })).toBe('apply')
  })

  it("'error' behaves like idle (clean ⇒ refresh, dirty ⇒ apply)", () => {
    expect(runButtonState({ phase: 'error', dirty: false, elapsedMs: 0, threshold: T })).toBe(
      'refresh'
    )
    expect(runButtonState({ phase: 'error', dirty: true, elapsedMs: 0, threshold: T })).toBe('apply')
  })

  it("'running' under threshold ⇒ busy, at/over threshold ⇒ cancel", () => {
    expect(runButtonState({ phase: 'running', dirty: false, elapsedMs: 100, threshold: T })).toBe(
      'busy'
    )
    expect(runButtonState({ phase: 'running', dirty: false, elapsedMs: 600, threshold: T })).toBe(
      'cancel'
    )
  })

  it("'validating' under threshold ⇒ busy, at/over threshold ⇒ cancel", () => {
    expect(runButtonState({ phase: 'validating', dirty: false, elapsedMs: 100, threshold: T })).toBe(
      'busy'
    )
    expect(runButtonState({ phase: 'validating', dirty: false, elapsedMs: 600, threshold: T })).toBe(
      'cancel'
    )
  })

  it('boundary: elapsedMs === threshold ⇒ cancel', () => {
    expect(runButtonState({ phase: 'running', dirty: false, elapsedMs: T, threshold: T })).toBe(
      'cancel'
    )
  })

  it('running ignores dirty: running + dirty + under threshold ⇒ busy', () => {
    expect(runButtonState({ phase: 'running', dirty: true, elapsedMs: 100, threshold: T })).toBe(
      'busy'
    )
  })
})
