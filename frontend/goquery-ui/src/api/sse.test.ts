import { describe, it, expect } from 'vitest'
import { SSEParser } from './sse'

const enc = new TextEncoder()
const bytes = (s: string) => enc.encode(s)

describe('SSEParser', () => {
  it('parses a single complete event', () => {
    const p = new SSEParser()
    const events = p.push(bytes('event: partial\ndata: {"a":1}\n\n'))
    expect(events).toEqual([{ event: 'partial', data: '{"a":1}' }])
  })

  it('returns nothing until the blank-line delimiter arrives', () => {
    const p = new SSEParser()
    expect(p.push(bytes('event: partial\ndata: {"a":1}\n'))).toEqual([])
    // the second \n completes the frame
    expect(p.push(bytes('\n'))).toEqual([{ event: 'partial', data: '{"a":1}' }])
  })

  it('reassembles an event split across byte chunks mid-field', () => {
    const p = new SSEParser()
    expect(p.push(bytes('event: fin'))).toEqual([])
    expect(p.push(bytes('al\ndata: {"x":'))).toEqual([])
    expect(p.push(bytes('1}\n\n'))).toEqual([{ event: 'final', data: '{"x":1}' }])
  })

  it('emits multiple framed events from one chunk', () => {
    const p = new SSEParser()
    const events = p.push(bytes('data: a\n\ndata: b\n\n'))
    expect(events).toEqual([{ data: 'a' }, { data: 'b' }])
  })

  it('joins multiple data lines with newlines', () => {
    const p = new SSEParser()
    const events = p.push(bytes('data: line1\ndata: line2\n\n'))
    expect(events).toEqual([{ data: 'line1\nline2' }])
  })

  it('normalizes CRLF to LF', () => {
    const p = new SSEParser()
    const events = p.push(bytes('event: partial\r\ndata: {"a":1}\r\n\r\n'))
    expect(events).toEqual([{ event: 'partial', data: '{"a":1}' }])
  })

  it('ignores comment lines', () => {
    const p = new SSEParser()
    const events = p.push(bytes(': keep-alive\ndata: x\n\n'))
    expect(events).toEqual([{ data: 'x' }])
  })

  it('flush surfaces a trailing unterminated block', () => {
    const p = new SSEParser()
    expect(p.push(bytes('event: final\ndata: {"x":1}'))).toEqual([])
    expect(p.flush()).toEqual([{ event: 'final', data: '{"x":1}' }])
  })

  it('flush returns nothing for empty trailing whitespace', () => {
    const p = new SSEParser()
    p.push(bytes('data: x\n\n'))
    expect(p.flush()).toEqual([])
  })

  it('flush is idempotent (drains the buffer)', () => {
    const p = new SSEParser()
    p.push(bytes('data: tail'))
    expect(p.flush()).toEqual([{ data: 'tail' }])
    expect(p.flush()).toEqual([])
  })
})
