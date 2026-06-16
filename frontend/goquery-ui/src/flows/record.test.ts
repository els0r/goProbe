import { describe, it, expect } from 'vitest'
import { flattenRow } from './record'
import { RowSchema } from '../api/domain'

const row = (counters: Record<string, number>): RowSchema =>
  ({ attributes: {}, counters, labels: {} } as unknown as RowSchema)

describe('flattenRow', () => {
  it('derives bytes_total and packets_total from the in/out counters', () => {
    const r = flattenRow(row({ br: 30, bs: 12, pr: 5, ps: 3 }))
    expect(r.bytes_total).toBe(42)
    expect(r.packets_total).toBe(8)
  })

  it('treats absent counters as zero totals', () => {
    const r = flattenRow(row({}))
    expect(r.bytes_total).toBe(0)
    expect(r.packets_total).toBe(0)
  })
})
