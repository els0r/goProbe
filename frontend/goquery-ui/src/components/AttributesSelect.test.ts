import { describe, it, expect } from 'vitest'
import { shownAttributes, parseAttributeQuery, ALLOWED_ATTR_ORDER } from './AttributesSelect'

describe('shownAttributes', () => {
  it('returns every allowed attribute in canonical order when all is set', () => {
    expect(shownAttributes({ values: [], all: true })).toEqual(['sip', 'dip', 'dport', 'proto'])
  })

  it('returns a fresh array that does not corrupt the shared order constant', () => {
    const out = shownAttributes({ values: [], all: true })
    out.push('mutated')
    expect(ALLOWED_ATTR_ORDER).toEqual(['sip', 'dip', 'dport', 'proto'])
  })

  it('returns the selected subset preserving caller order', () => {
    expect(shownAttributes({ values: ['dip', 'sip'], all: false })).toEqual(['dip', 'sip'])
  })

  it('does not alias the result back into the caller values array', () => {
    const values = ['dip', 'sip']
    const out = shownAttributes({ values, all: false })
    out.push('mutated')
    expect(values).toEqual(['dip', 'sip'])
  })

  // Dead-code guard: parseAttributeQuery already canonicalizes aliases, so the
  // old inline `v === 'protocol' ? 'proto' : ...` map at the call sites was a
  // no-op. Proven here so it stays deleted.
  it('receives already-canonical values from parseAttributeQuery (no aliasing needed)', () => {
    const parsed = parseAttributeQuery('port,protocol')
    expect(parsed.values).toEqual(['dport', 'proto'])
    expect(shownAttributes(parsed)).toEqual(['dport', 'proto'])
  })
})
